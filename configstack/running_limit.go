package configstack

import (
	"fmt"
	"time"

	"github.com/fatih/color"
)

const waitTimeBetweenThread = 2500

func initWorkers(n int) {
	if n <= 0 {
		panic(fmt.Errorf("the number of workers must be greater than 0 (%d)", n))
	}
	if burstyLimiter == nil {
		burstyLimiter = make(chan int, n)
		for i := 1; i <= n; i++ {
			fmt.Println("Adding token", i, "to the channel")
			burstyLimiter <- i
			time.Sleep(waitTimeBetweenThread * time.Millisecond) // Start workers progressively to avoid throttling
		}
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
	writer := module.Module.TerragruntOptions.Logger.Infof
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
