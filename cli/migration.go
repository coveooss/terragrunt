package cli

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime/debug"
	"strings"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	gotemplateHcl "github.com/coveooss/gotemplate/v3/hcl"
	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/config"
	"github.com/gruntwork-io/terragrunt/util"
	"github.com/lithammer/dedent"
	"github.com/sergi/go-diff/diffmatchpatch"
	"github.com/urfave/cli"
	kingpin "gopkg.in/alecthomas/kingpin.v2"
)

var terragruntConfigRegex = regexp.MustCompile(`terragrunt\s*=?\s*{`)

// Variable regex
var variableRegex = regexp.MustCompile(`-VAR-(\w+)`)

// https://regex101.com/r/B3grTb/1
var variableInNameRegex = regexp.MustCompile(`((?:provider|resource|data).*){{\s*\$(\w+)\s*}}(.*)`)

// https://regex101.com/r/YqfYli/1
var variableRazorInNameRegex = regexp.MustCompile(`((?:provider|resource|data).*)@{(\w+)}(.*)`)

// run_if and ignore_if are now attributes, not blocks
var runIfRegex = regexp.MustCompile(`run_if\s*{`)
var ignoreIfRegex = regexp.MustCompile(`ignore_if\s*{`)

func migrate(cliContext *cli.Context) (err error) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Println("stacktrace from panic: \n" + string(debug.Stack()))
		}
	}()

	app := kingpin.New("terragrunt 0.12upgrade", "Upgrade your Terraform files from the format used in Terraform 0.11 to the one used in 0.12")
	workingDirectory, _ := os.Getwd()
	targetDirectory := app.Flag("target-directory", "Where your Terraform files are located").Short('t').Default(workingDirectory).String()
	write := app.Flag("write", "Write the changes to your files. Otherwise, display a diff").Short('w').Bool()
	app.Flag("terragrunt-logging-level", "").String()
	kingpin.MustParse(app.Parse(cliContext.Args()[1:]))

	// Keep a copy of the original files (for a diff at the end)
	copyOfOriginalsDir, err := copyToTemporaryDirectory("origin-copy", *targetDirectory)
	defer os.RemoveAll(copyOfOriginalsDir)
	if err != nil {
		return err
	}

	// If write is not set, do the work in a temp dir
	if write == nil || !*write {
		tempMigrationDir, err := copyToTemporaryDirectory("migration", *targetDirectory)
		defer os.RemoveAll(tempMigrationDir)
		if err != nil {
			return err
		}

		*targetDirectory = tempMigrationDir
	}

	// 1. Rewrite gotemplate -VAR-<variable> to GOVAR_<variable>_ENDVAR
	if err := forEachFile(*targetDirectory, func(fullPath, relativePath string) error {
		content, err := util.ReadFileAsString(fullPath)
		if err != nil {
			return err
		}
		content = variableRegex.ReplaceAllString(content, "GOVAR_${1}_ENDVAR")
		content = variableInNameRegex.ReplaceAllString(content, "${1}GOVAR_${2}_ENDVAR${3}")
		content = variableRazorInNameRegex.ReplaceAllString(content, "${1}GOVAR_${2}_ENDVAR${3}")
		return ioutil.WriteFile(fullPath, []byte(content), 0666)
	}); err != nil {
		return err
	}

	// 2. Run terraform 0.12upgrade
	if err := forEachFolder(*targetDirectory, func(path string) error {
		filesInDir, err := ioutil.ReadDir(path)
		if err != nil {
			return err
		}
		for _, fileInDir := range filesInDir {
			if strings.HasSuffix(fileInDir.Name(), ".tf") {
				command := exec.Command("terraform", "init")
				command.Dir = path
				// If the command fails, the reasons are too many to try to resolve the situation, let's just try without the init
				command.Run()
				defer os.RemoveAll(filepath.Join(path, ".terraform"))

				command = exec.Command("terraform", "0.12upgrade", "-yes")
				command.Dir = path
				command.Env = []string{"TF_LOG=DEBUG"}
				if output, err := command.CombinedOutput(); err != nil {
					fmt.Println(string(output))
					return fmt.Errorf("terraform upgrade error in path %s: %v", path, err)
				}
				break
			}
		}
		return nil
	}); err != nil {
		return err
	}

	// 3. Rewrite Terragrunt files from terraform.tfvars to terragrunt.hcl
	if err := forEachFile(*targetDirectory, migrateConfigurationFile); err != nil {
		return err
	}

	// 4. Remove flattened variables
	allProjects := []string{"infra/*"}
	if err := forEachFile(*targetDirectory, func(fullPath, relativePath string) error {
		if strings.HasSuffix(fullPath, config.DefaultConfigName) {
			var configFileMap map[string]interface{}
			configFile, err := ioutil.ReadFile(fullPath)
			if err != nil {
				return err
			}
			if err := gotemplateHcl.Unmarshal(configFile, &configFileMap); err != nil {
				return err
			}

			if inputs, ok := configFileMap["inputs"]; ok {
				inputsMap := inputs.(map[string]interface{})
				if projects, ok := inputsMap["import_projects"]; ok {
					projectsList := projects.([]interface{})
					for _, project := range projectsList {
						if value, ok := project.(string); ok {
							allProjects = util.RemoveDuplicatesFromListKeepFirst(append(allProjects, value))
						}
					}
				}
			}
		}
		return nil
	}); err != nil {
		return err
	}
	flattenedVariablesReplacements := getAllFlattenedReplacementsFromImportedProjects(allProjects)
	if err := forEachFile(*targetDirectory, func(fullPath, relativePath string) error {
		originalContent, err := util.ReadFileAsString(fullPath)
		if err != nil {
			return err
		}
		var content string = originalContent
		for toReplace, replacement := range flattenedVariablesReplacements {
			content = strings.ReplaceAll(content, "local_"+toReplace, "local."+replacement)
			content = strings.ReplaceAll(content, "main_"+toReplace, "main."+replacement)
			content = strings.ReplaceAll(content, toReplace, replacement)
		}
		if content != originalContent {
			return util.WriteFileWithSamePermissions(fullPath, fullPath, []byte(content))
		}
		return nil
	}); err != nil {
		return err
	}

	// Print diffs for all files that were modified
	// Also delete "versions.tf" files written by the previous step
	if err := forEachFile(*targetDirectory, func(fullPath, relativePath string) error {
		originPath := relativePath
		if strings.HasSuffix(fullPath, config.DefaultConfigName) {
			originPath = filepath.Join(filepath.Dir(relativePath), "terraform.tfvars")
		}
		originContent, err := util.ReadFileAsString(filepath.Join(copyOfOriginalsDir, originPath))
		if err != nil && strings.HasSuffix(originPath, "versions.tf") {
			// delete versions.tf files
			return os.Remove(fullPath)
		} else if err != nil {
			return err
		}
		newContent, err := util.ReadFileAsString(fullPath)
		if err != nil {
			return err
		}
		printFileDiff(originPath, originContent, relativePath, newContent)
		return nil
	}); err != nil {
		return err
	}

	return nil
}

// * Turns terraform.tfvars files into terragrunt.hcl
// * Adds equal sign to run_condition's `run_if` and `ignore_if` statements
func migrateConfigurationFile(fullPath, relativePath string) error {
	if !strings.HasSuffix(fullPath, "terraform.tfvars") {
		return nil
	}

	content, err := util.ReadFileAsString(fullPath)
	if err != nil {
		return err
	}
	terragruntStatement := terragruntConfigRegex.FindStringIndex(content)

	newContent := ""
	if terragruntStatement != nil {
		terragruntConfigDefinitionStart, terragruntBlockStart := terragruntStatement[0], terragruntStatement[1]

		// Find terragrunt block end
		index := terragruntBlockStart
		countOpen, countClose := 1, 0
		for countOpen != countClose {
			if content[index] == '{' {
				countOpen++
			} else if content[index] == '}' {
				countClose++
			}
			index++
		}
		terragruntBlockEnd := index
		newContent = strings.TrimSpace(dedent.Dedent(content[terragruntBlockStart : terragruntBlockEnd-1]))
		content = content[:terragruntConfigDefinitionStart] + content[terragruntBlockEnd:]
	}

	var inputVariables map[string]interface{}
	if err := gotemplateHcl.Unmarshal([]byte(content), &inputVariables); err != nil {
		return fmt.Errorf("Error unmarshalling with gotemplate: %v", err)
	}
	if len(inputVariables) > 0 {
		inputVariablesMarshalled, err := gotemplateHcl.MarshalTFVarsIndent(map[string]interface{}{"inputs": inputVariables}, "  ", "  ")
		if err != nil {
			return fmt.Errorf("error marshalling input variables: %v", err)
		}
		newContent += "\n" + string(inputVariablesMarshalled)
	}

	// Add equal sign to run_if and ignore_if
	newContent = runIfRegex.ReplaceAllString(newContent, "run_if = {")
	newContent = ignoreIfRegex.ReplaceAllString(newContent, "ignore_if = {")

	newPath := filepath.Join(filepath.Dir(fullPath), config.DefaultConfigName)
	if err := ioutil.WriteFile(newPath, []byte(newContent), 0666); err != nil {
		return err
	}
	if err := os.Chmod(newPath, 0666); err != nil {
		return err
	}

	return os.Remove(fullPath)
}

func forEachFile(directory string, action func(fullPath, relativePath string) error) error {
	return filepath.Walk(directory, func(walkPath string, info os.FileInfo, err error) error {
		if info == nil || info.IsDir() || err != nil {
			return nil
		}
		if strings.HasPrefix(info.Name(), ".") {
			return nil
		}
		relativePath, _ := filepath.Rel(directory, walkPath)
		return action(walkPath, relativePath)
	})
}

func forEachFolder(directory string, action func(fullPath string) error) error {
	return filepath.Walk(directory, func(walkPath string, info os.FileInfo, err error) error {
		if !info.IsDir() || err != nil {
			return nil
		}
		return action(walkPath)
	})
}

func printFileDiff(oldFilename, oldContent, newFilename, newContent string) {
	titleStyle := color.New(color.Bold, color.Underline)
	if oldFilename != newFilename {
		filenameDmp := diffmatchpatch.New()
		filenameDiffs := filenameDmp.DiffCleanupSemantic(filenameDmp.DiffMain(color.YellowString(oldFilename), color.YellowString(newFilename), false))
		title := filenameDmp.DiffPrettyText(filenameDiffs)
		titleStyle.Println(title)
	} else if newContent != oldContent {
		title := color.YellowString(oldFilename)
		titleStyle.Println(title)
	}

	if newContent != oldContent {
		dmp := diffmatchpatch.New()
		diffs := dmp.DiffCleanupSemantic(dmp.DiffMain(oldContent, newContent, false))
		fmt.Println(dmp.DiffPrettyText(diffs))

	}
	fmt.Println()
}

func copyToTemporaryDirectory(name string, source string) (string, error) {
	tempDir, err := ioutil.TempDir("", "terragrunt-0.12-"+name)
	if err != nil {
		return "", err
	}
	return tempDir, util.CopyFolderContents(source, tempDir)
}

func getAllFlattenedReplacementsFromImportedProjects(importedProjects []string) map[string]string {
	replacements := map[string]string{}
	session := session.Must(session.NewSession())
	svc := s3.New(session)
	if err := svc.ListObjectsPages(&s3.ListObjectsInput{
		Bucket: aws.String("coveo-terraform-outputs-us-east-1"),
	}, func(p *s3.ListObjectsOutput, last bool) (shouldContinue bool) {
		for _, obj := range p.Contents {
			// Format of keys is env/region/project/subproject.tfvars
			key := *obj.Key
			if strings.HasSuffix(key, ".tfvars") {
				for _, importedProject := range importedProjects {
					splitImported := strings.Split(importedProject, "/")
					project := splitImported[0]
					subproject := ""
					if len(splitImported) == 2 {
						if value := splitImported[1]; value != "*" {
							subproject = value
						}
					}

					splitKey := strings.Split(key, "/")
					objProject := splitKey[2]
					objSubproject := strings.Split(splitKey[3], ".")[0]
					if objProject == project && (subproject == "" || subproject == objSubproject) {
						objProject = strings.Replace(objProject, "-", "_", -1)
						objSubproject = strings.Replace(objSubproject, "-", "_", -1)
						replacements[objProject+"_"+objSubproject+"_"] = objProject + "." + objSubproject + "."
					}
				}
			}
		}
		return true
	}); err != nil {
		panic(err)
	}
	return replacements
}
