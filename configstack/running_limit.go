package configstack

import (
	"fmt"
	"time"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/util"
)

const waitTimeBetweenThread = 2500
const defaultWorkersLimit = 10

func initWorkers(n int) {
	if n <= 0 {
		panic(fmt.Errorf("The number of workers must be greater than 0 (%d)", n))
	}
	burstyLimiter = make(chan int, n)
	for i := 1; i <= n; i++ {
		time.Sleep(waitTimeBetweenThread * time.Millisecond) // Start workers progressively to avoid throttling
		burstyLimiter <- i
	}
}

func nbWorkers() int       { return cap(burstyLimiter) }
func waitWorker() int      { return <-burstyLimiter }
func freeWorker(token int) { burstyLimiter <- token }

var burstyLimiter chan int

// OutputPeriodicLogs displays current module output for long running request
func (module *runningModule) OutputPeriodicLogs(completed *bool) {
	if module.Module.TerragruntOptions.RefreshOutputDelay == 0 {
		return
	}
	writer := module.Module.TerragruntOptions.Writer.(util.LogCatcher).Logger.Noticef
	for {
		time.Sleep(module.Module.TerragruntOptions.RefreshOutputDelay)
		if *completed {
			break
		}
		partialOutput := module.OutStream.String()
		if len(partialOutput) > module.bufferIndex {
			end := len(partialOutput)
			partialOutput = partialOutput[module.bufferIndex:end]
			message := color.New(color.FgHiCyan).Sprintf("Still waiting for task to complete\n%s\n%s (partial output):\n", separator, module.displayName())
			writer("%s\n%s\n", message, partialOutput)
			module.bufferIndex = end
		}
	}
}
