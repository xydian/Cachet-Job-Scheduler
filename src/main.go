package main

import (
	"github.com/BurntSushi/toml"
	"log"
	"os"
	//"reflect"
	// "fmt"
	"github.com/andygrunwald/cachet"
	//"io/ioutil"
	//"os/exec"
	"os/signal"
	"strconv"
	"sync"
	"syscall"
	"time"
	//"errors"
	"CachetJobScheduler/src/pkgs/jobscheduler"
)

var debugMode bool
var executeOnlySingleJob string
var logPath string
var generalLogFile string
var jobLogDirectory string
var config jobscheduler.Config
var configFilePath string
var cachetClient *cachet.Client
var logFile *os.File

// Reading config files, creating/opening log files, connecting to cachet instance
func init() {
	configFilePath = "/etc/CachetJobScheduler/config.toml"
	logPath = "/var/log/CachetJobScheduler"
	generalLogFile = logPath + "/jobscheduler.log"
	jobLogDirectory = logPath + "/jobs"

	var err error
	// setting default properties, which can be overwritten by command line arguments
	debugMode = false
	// if this is set, only the job specified (by its name) in the command line argument is executed
	executeOnlySingleJob = ""

	// Reading and setting command line options
	commandArguments := os.Args[1:]
	for index, option := range commandArguments {
		if option == "-d" {
			debugMode = true
		}
		// execute only single job
		if option == "-j" {
			executeOnlySingleJob = commandArguments[index+1]
		}
		// specify another logfile
		if option == "-lf" {
			generalLogFile = commandArguments[index+1]
		}
	}

	// if you want to test the execution of a job i guess its used for debugging
	// so debug mode is enabled
	if executeOnlySingleJob != "" {
		debugMode = true
	}

	// Trying to open general logfile
	logFile, err := os.OpenFile(generalLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
	if err != nil {
		log.Fatalf("Error opening general logfile: %v", err)
	}

	// Setting log output to the general logfile, but only if debug mode is disabled
	if !debugMode {
		log.Println("Debug mode is disabled. Logging to folder " + logPath)
		log.Println("General logs will be written to jobscheduler.log. Job specific " +
			"logs are written to their respective log files")
		log.SetOutput(logFile)
	}

	log.Println("Checking whether log directory for jobs exists...")
	if _, err = os.Stat(jobLogDirectory); err != nil {
		if os.IsNotExist(err) {
			// Folder does not exist
			log.Println("Creating jobs directory in log path")
			err = os.Mkdir(jobLogDirectory, 0774)
			if err != nil {
				log.Fatalf("Error creating log directory for jobs: %v", err)
			}
		} else {
			log.Fatalf("Error while checking whether log directory for jobs exists: %v", err)
		}
	} else {
		log.Println("Log directory for jobs already exists, skipping creation")
	}

	log.Println("Reading config file...")
	if _, err = toml.DecodeFile(configFilePath, &config); err != nil {
		log.Fatalf("Error reading config file %v: %v", configFilePath, err)
	}
	if debugMode {
		log.Println("Read config file, parsed the following: ")
		config.Print()
	}

	config.FillNameStructField()

	log.Println("Checking configuration...")
	for _, job := range config.Jobs {
		configErr := job.CheckConfig()
		if configErr != nil {
			log.Fatal("Job '" + job.Name + "' configuration error: " + configErr.Error())
		}
	}

	log.Println("Opening (respectively creating) separate log files for each job...")
	for jobName, jobConfig := range config.Jobs {
		//log.Println("Opening (or creating) log file for job: " + jobName)
		printDebugLog("Opening (or creating) log file for job: " + jobName)

		jobLogFile := jobLogDirectory + "/" + jobName + ".log"
		f, err := os.OpenFile(jobLogFile, os.O_RDWR|os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			log.Fatalf("Error opening logfile of job %v: %v", jobName, err)

		} else {
			jobConfig.LogFile = f
			config.Jobs[jobName] = jobConfig
		}
		//defer f.Close()
	}

	log.Println("Trying to connect to Cachet instance...")
	cachetClient, _ = cachet.NewClient(config.Host, nil)
	_, resp, err := cachetClient.General.Ping()

	cachetClient.Authentication.SetTokenAuth(config.APIToken)

	if err != nil {
		log.Fatal("Cachet instance can´t be reached on the configured host '" + config.Host + "': " + err.Error())
	}
	if resp.StatusCode != 200 {
		log.Fatal("Cachet API doesn´t seem to be working. " +
			"The returned status code should be 200. Instead it was: " +
			strconv.Itoa(resp.StatusCode))
	}

	log.Println("Successfully connected to Cachet API on: " + config.Host)

	if executeOnlySingleJob != "" {
		log.Println("You specified to only run the job '" + executeOnlySingleJob +
		"' by using the -j option, so all other jobs are going to be disabled...")

		jobFound := false
		for jobName, jobConfig := range config.Jobs {
			if jobName == executeOnlySingleJob {
				jobConfig.Enabled = true
				config.Jobs[jobName] = jobConfig
				jobFound = true
			} else {
				jobConfig.Enabled = false
				config.Jobs[jobName] = jobConfig
			}
		}
		if !jobFound {
			log.Fatal("The specified job was not found")
		} else {
			log.Println("All jobs except '" + executeOnlySingleJob + "' are disabled")
		}
	}
}

func main() {
	//var err error

	log.Println("JobScheduler has been started. Starting to execute jobs...")

	// When SIGINT or SIGTERM is caught write to the quitChannel
	quitSigChannel := make(chan os.Signal)
	signal.Notify(quitSigChannel, syscall.SIGINT, syscall.SIGTERM, syscall.SIGKILL)

	// making channel to stop the jobs (goroutines)
	quitGoroutinesChannel := make(chan bool)

	// when quitting we must wait for all jobs to stop, hence the waitgroup
	var waitgroup sync.WaitGroup
	waitgroup.Add(len(config.Jobs))

	for _, job := range config.Jobs {
		if !job.Enabled {
			log.Println("Job '" + job.Name + "' is disabled. Skipping execution")
			// This job is not going to be executed. Hence it is 'done'
			waitgroup.Done()
		} else {
			//waitgroup.Add(1)
			go func(job jobscheduler.Job, cachetClient *cachet.Client) {
				ticker := time.NewTicker(time.Duration(job.NextCheckDelay) * time.Minute)
				defer ticker.Stop()

				// This anonymous function is kind of ugly, but necessary as we need to
				// execute the job once at the beginning and then each time when the
				// ticker channel notifies.
				// This helper function was written in favor of code duplication
				jobExecution := func() {
					/*var jobError error
					jobError = nil
					executionTriesCounter := 0

					for jobError != nil | executionTriesCounter <= job.NumberOfExecutionAttempts {

					}*/

					_, jobError := job.Execute()
					if jobError != nil {
						compID := job.CachetComponentID
						//log.Println(cachetClient.Components.)
						comp, resp, err := cachetClient.Components.Get(compID)
						if err != nil {
							log.Println(err)
							log.Fatal()
						}
						if resp.StatusCode == 200 {
							job.WriteLog("Current component status: " + comp.StatusName)
							// check if component status is already in failed state
							if comp.Status != job.CachetStatus {
								comp.Status = job.CachetStatus
								_, resp, _ = cachetClient.Components.Update(job.CachetComponentID, comp)
								if resp.StatusCode == 200 {
									job.WriteLog("Component set to statuscode: " + strconv.Itoa(job.CachetStatus))
								} else {
									logMessage := "Could not update component in Cachet. Returned statuscode is: " + resp.Status
									job.WriteLog(logMessage)
									log.Fatal(job.Name + ": " + logMessage)
								}
							} else {
								job.WriteLog("Status of component is already set to the one that is configured " +
									"in case of job execution failure, no need to update")
							}
						}
					}
					job.WriteLog("Next Check in " + (time.Duration(job.NextCheckDelay) * time.Minute).String())
				}
				// Execute once
				jobExecution()

				// Execute periodically after configured time (NextCheckDelay)
				for {
					select {
					case _ = <-quitGoroutinesChannel:
						waitgroup.Done()
						log.Println("Job '" + job.Name + "' stopped")
						return
					case <-ticker.C:
						jobExecution()
					}
				}
			}(job, cachetClient)
			log.Println("Execution of job '" + job.Name + "' has been started")
		}
	}

	if executeOnlySingleJob == "" {
		log.Println("All jobs have been started")
	} else {
		log.Println("Configured job has been started, log is found in " +
			config.Jobs[executeOnlySingleJob].LogFile.Name())
	}

	// wait until we get SIGINT or SIGTERM e.g. by pressing ctrl + c
	<-quitSigChannel

	log.Println("Jobscheduler received SIGINT/SIGTERM. Stopping jobs...")

	quitGoroutinesChannel <- true

	// close channel
	close(quitGoroutinesChannel)

	// wait for all go routines to stop
	waitgroup.Wait()
	log.Println("All jobs have been stopped")

	// closing files
	printDebugLog("Closing general log file (jobscheduler.log)...")
	err := logFile.Close()
	if err != nil {
		printDebugLog(err.Error())
	}
}

func printDebugLog(logString string) {
	if debugMode {
		log.Println(logString)
	}
}
