# CachetJobScheduler

Cachet Job Scheduler lets you define different monitoring jobs (Scripts/commands
  that should be run in specific time interval). If a monitoring job fails the tool is
  setting a configured component of your cachet instance to a specified status.

  The tool runs the scripts/commands periodically and checks their exit code.
  If they exit with an exit code greater than 0 the corresponding component is
  assumed to be down/not functioning and the tool updates the status of the component

### Features
- Run custom monitoring scripts/commands periodically
- Update component of your Cachet instance if one of the monitoring jobs fails
- Set a maximum execution time for each script/command
- Configure command arguments for each script/command

### Config
I created a sample config file with some simple jobs to monitor a website using
  wget command and a job to check if a nameserver is respondig using dig command

#### Registering a new monitoring job
To create a new job you have to ad a new job to section 'Jobs' in jobscheduler.toml

```toml
  # simple website monitor job, checking if google website is up
  [Jobs.CheckGoogleWebsite] # this is the single identifier of each job
  Enabled = true
  # this is the command that is going to be executed periodically
  # this job is testing whether the google front page can be reached
  Command_to_execute = "wget"
  # options that are passed to the executed command
  Command_options = ["google.com", "--spider"]
  # minutes to the next time the scheduler runs the comman
  Next_check_delay = 5
  # if the command/script exceeds its execution time by this value in seconds
  # the scheduler will abort the execution and set the component status to the
  # one configured below
  Max_executing_time = 7
  # the id of the component in cachet that is monitored by this job
  Cachet_component_id = 1
  # status to set when script/command execution fails (Exit code > 0)
  Cachet_status = 3
  # create an incident for the specified component (not yet implemented)
  Create_incident_on_failure = true
  # working directory of the script/command
  Working_directory = ""
  # number of execution retires of command, if the command still fails (Exit
  # code > 0) after the number of attempts below,
  # the component status is updated in Cachet
  Number_of_execution_retries = 1
  # delay in seconds between the execution retries. In this case the wget command
  # is executed after 5 seconds
  Execution_retry_delay = 5
```

#### Testing new monitoring jobs

You can test the configuration and execution of new jobs by running CachetJobScheduler -j <jobname>. The JobScheduler will then run only this job, logs can be found on /var/log/CachetJobScheduler/jobs/<jobname>.log

### Setup
- Get the current version of CachetJobScheduler from [releases](https://github.com/xydian/Cachet-Job-Scheduler/releases)
- Create /etc/CachetJobScheduler
- Place config.toml in the created directory
- Create /var/log/CachetJobScheduler and ensure the user you use to run the
tool has write permissions to it
- Run CachetJobScheduler

### Starting/Stopping the daemon
You have to register the CachetJobScheduler.service file at systemd. On debian you have to
place the .service file at /lib/systemd/system/CachetJobScheduler.service. Then
you can enable the service by the following command:

`sudo systemctl enable CachetJobScheduler.service`

Now you can start/stop the service by executing:

`sudo systemctl CachetJobScheduler start/stop`

### CachetJobScheduler arguments
- -d enables debug mode
- use -j <jobname> to only run the specified job. This also enables debug mode.
- use -lf <filepath> to specify a custom log file
