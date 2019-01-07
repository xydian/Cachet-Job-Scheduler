package jobscheduler

import (
  "os"
  "log"
  "reflect"
  "errors"
  "time"
  "os/exec"
  "io/ioutil"
)

// Config contains basic config for the JobScheduler Instance
type Config struct {
	Host     string
	APIToken string `toml:"Api_Token"`
	// TODO: email server settings to send mail if jobscheduler stops
	/*Default_next_check_delay           int
	Default_max_execution_time         int
	Default_cachet_status              int
	Default_create_incident_on_failure bool*/
	Jobs map[string]Job `toml:"Jobs"`
}

// FillNameStructField fills the still empty struct field name of each job
// of JobSchedulerConfig as the toml parser doesn´t do this
func (config *Config) FillNameStructField(){
	// Fill name struct field
	for jobName, jobConfig := range config.Jobs {
		jobConfig.Name = jobName
		config.Jobs[jobName] = jobConfig
	}
}

// Print prints the config to the log in a readable format
func (config *Config) Print() {
	log.Println("Host: " + config.Host)
	log.Println("Api_Token: " + config.APIToken)

	log.Println("Printing configured jobs: ")
	for jobName, jobConfig := range config.Jobs {
		log.Println("- " + jobName)

		s := reflect.ValueOf(&jobConfig).Elem()
		typeOfT := s.Type()

		for i := 0; i < s.NumField(); i++ {
			f := s.Field(i)
			log.Printf("    %s %s = %v\n",
				typeOfT.Field(i).Name, f.Type(), f.Interface())
		}
	}
}

// Job contains all information needed to executed a monitoring job
type Job struct {
	Enabled                 bool     `toml:"Enabled"`
	PathToScript            string   `toml:"Command_to_execute"`
	WorkingDirectory        string   `toml:"Working_directory"`
	NextCheckDelay          int      `toml:"Next_check_delay,omitempty"`
	MaxExecutingTime        int      `toml:"Max_executing_time,omitempty"`
	CachetComponentID       int      `toml:"Cachet_component_id"`
	CachetStatus            int      `toml:"Cachet_status,omitempty"`
	CreateIncidentOnFailure bool     `toml:"Create_incident_on_failure,omitempty"`
	Options                 []string `toml:"Command_options"`
	LogFile                 *os.File
	Name                    string
}

// CheckConfig checks whether the configuration of a job has valid values.
// However it doesn´t check whether the script to execute exists.
func (job *Job) CheckConfig() error {
	// TODO: Check config, for example whether maxexecutingtime is greater than zero
	if job.MaxExecutingTime <= 0 {
		return errors.New("Max_executing_time has to be greater than 0")
	}
	if job.CachetComponentID <= 0 {
		return errors.New("Cachet_component_id has to be greater than 0")
	}
	if job.CachetStatus <= 0 {
		return errors.New("Cachet_status has to be greater than 0")
	}
	if job.NextCheckDelay <= 0 {
		return errors.New(("Next_check_delay has to be greater than 0"))
	}
	if job.PathToScript == "" {
		return errors.New("Path_to_script is not allowed to be empty")
	}

	return nil
}

// WriteLog writes the log to the logfile for the job
func (job *Job) WriteLog(log string) {
	job.LogFile.WriteString(time.Now().Format("2006/01/02 15:04:05") + " " +
		log + "\r\n")
}

// Execute executes a job based on the job config
func (job *Job) Execute() (string, error) {
	// TODO: execute job
	executionTimer := time.Now()

	job.WriteLog("Starting to execute job")
	cmd := exec.Command(job.PathToScript, job.Options...)
	if job.WorkingDirectory != "" {
		cmd.Dir = job.WorkingDirectory
	}

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		log.Fatal(err)
	}

	err = cmd.Start()
	if err != nil {
		// file not found
		job.WriteLog("The file to be executed was not found on '" + job.PathToScript + "'")
		log.Fatal(err)
	}

	// Kill the process after x seconds
	timer := time.NewTimer(time.Second * time.Duration(job.MaxExecutingTime))
	go func(timer *time.Timer, cmd *exec.Cmd) {
		for _ = range timer.C {
			err = cmd.Process.Signal(os.Kill)
			if err != nil {
				//os.Stderr.WriteString(err.Error())
				//job.WriteLog("Error stopping job: " + err.Error())
			} else {
				job.WriteLog("The job did take longer than configured in job config (Max_execution_time) " +
					"Execution has been aborted.")
			}
		}
	}(timer, cmd)

	output, _ := ioutil.ReadAll(stdout)
	err = cmd.Wait()

	jobOutput := string(output)

	if err != nil {
		job.WriteLog("The job didn´t execute successfully")
		job.WriteLog("output of job: " + jobOutput)
	} else {
		job.WriteLog("Job execution successfull (" + time.Since(executionTimer).String() + ")")
	}

	job.WriteLog("Next Check in " + (time.Duration(job.NextCheckDelay) * time.Minute).String())

	return jobOutput, err
}
