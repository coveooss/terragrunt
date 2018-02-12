package configstack

import (
	"time"

	"github.com/fatih/color"
	"github.com/gruntwork-io/terragrunt/util"
)

const (
	maxThreadSimultaneousLaunch = 10
	waitTimeBetweenThread       = 2
)

func initSlowDown() {
	burstyLimiter = make(chan int, maxThreadSimultaneousLaunch)
	for i := 0; i < maxThreadSimultaneousLaunch; i++ {
		burstyLimiter <- i
	}

	go func() {
		// Help avoiding all treads trying to start at the same moment
		for _ = range time.Tick(waitTimeBetweenThread * time.Second) {
			burstyLimiter <- -1
		}
	}()
}

func slowDown() { <-burstyLimiter }

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
			writer("%s\n%s", color.CyanString("Still waiting for task to complete, partial output:"), partialOutput)
			module.bufferIndex = end
		}
	}
}
