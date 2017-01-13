/*******************************************************************************
*
* Copyright 2016 SAP SE
*
* Licensed under the Apache License, Version 2.0 (the "License");
* you may not use this file except in compliance with the License.
* You should have received a copy of the License along with this
* program. If not, you may obtain a copy of the License at
*
*     http://www.apache.org/licenses/LICENSE-2.0
*
* Unless required by applicable law or agreed to in writing, software
* distributed under the License is distributed on an "AS IS" BASIS,
* WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
* See the License for the specific language governing permissions and
* limitations under the License.
*
*******************************************************************************/

package main

import (
	"os"
	"os/signal"
	"sync"
	"syscall"

	"golang.org/x/net/context"

	"github.com/ncw/swift"
)

//SharedState contains all the stuff shared between all worker threads.
type SharedState struct {
	Configuration
	Context         context.Context
	SwiftConnection *swift.Connection
	WaitGroup       sync.WaitGroup

	//each of these is only ever written by one thread (and then read by the
	//main thread after waiting on the writing thread), so no additional
	//locking is required for these fields
	DirectoriesScanned uint64
	FilesFound         uint64
	FilesNeedTransfer  uint64
	FilesTransferred   uint64
}

//Run starts and orchestrates the various worker threads.
func Run(state *SharedState) {
	//receive SIGINT/SIGTERM signals
	sigs := make(chan os.Signal, 1)
	signal.Notify(sigs, os.Interrupt, syscall.SIGTERM)

	//setup a context that cancels the workers when one of the signals above is received
	var cancelFunc func()
	state.Context, cancelFunc = context.WithCancel(state.Context)
	defer cancelFunc()
	go func() {
		<-sigs
		cancelFunc()
	}()

	//setup a simple linear pipeline of workers (it should be fairly trivial to
	//scale this out to multiple workers later)
	makeTransferThread(state, makeCheckerThread(state, makeScraperThread(state)))

	//wait for all of them to return
	state.WaitGroup.Wait()

	//report results
	Log(LogInfo, "%d dirs scanned, %d files found, %d/%d files transferred",
		state.DirectoriesScanned, state.FilesFound,
		state.FilesNeedTransfer, state.FilesTransferred,
	)
}

func makeScraperThread(state *SharedState) <-chan File {
	state.WaitGroup.Add(1)
	out := make(chan File, 10)

	scraper := NewScraper(&state.Configuration)

	go func() {
		defer state.WaitGroup.Done()
		defer close(out)

		var directoriesScanned uint64
		var filesFound uint64

		for {
			//check if state.Context.Done() is closed
			if state.Context.Err() != nil {
				break
			}
			if scraper.Done() {
				break
			}

			for _, file := range scraper.Next() {
				filesFound++
				out <- file
			}
			directoriesScanned++
		}

		//submit statistics to main thread
		state.DirectoriesScanned = directoriesScanned
		state.FilesFound = filesFound
	}()

	return out
}

func makeCheckerThread(state *SharedState, in <-chan File) <-chan File {
	state.WaitGroup.Add(1)
	out := make(chan File, 10)
	done := state.Context.Done()

	go func() {
		defer state.WaitGroup.Done()
		defer close(out)

		var filesNeedTransfer uint64

	WorkerLoop:
		for {
			var file File
			select {
			case <-done:
				break WorkerLoop
			case file = <-in:
				if file.Path == "" {
					//input channel is closed and returns zero values
					break WorkerLoop
				}
				if file.NeedsTransfer(state.SwiftConnection) {
					filesNeedTransfer++
					out <- file
				}
			}
		}

		//submit statistics to main thread
		state.FilesNeedTransfer = filesNeedTransfer
	}()

	return out
}

func makeTransferThread(state *SharedState, in <-chan File) {
	state.WaitGroup.Add(1)
	done := state.Context.Done()

	go func() {
		defer state.WaitGroup.Done()

		var filesTransferred uint64

	WorkerLoop:
		for {
			var file File
			select {
			case <-done:
				break WorkerLoop
			case file = <-in:
				if file.Path == "" {
					//input channel is closed and returns zero values
					break WorkerLoop
				}
				if file.PerformTransfer(state.SwiftConnection) {
					filesTransferred++
				}
			}
		}

		//submit statistics to main thread
		state.FilesTransferred = filesTransferred
	}()
}