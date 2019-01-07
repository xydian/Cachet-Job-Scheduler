# CachetJobScheduler

Cachet Job Scheduler lets you define different monitoring jobs (Scripts/commands
  that should be run periodically). If a monitoring job fails the tool is
  setting a configured component of your cachet instance to a specified status.

  The tool runs the scripts/commands periodically and checks their exit code.
  If they exit with an exit code greater than 0 the corresponding component is
  assumed to be down/not functioning and the tool sets the status of the component

### Features
- Run custom monitoring scripts/commands periodically
- Update component of your Cachet instance if one of the monitoring jobs fails
- Set a maximum execution time for each script/command
- Configure command arguments for each script/command

### Config
I created a sample config file with some simple jobs to monitor a website using
  wget command and a job to check if a nameserver is respondig using dig command

### Setup
- Create /etc/CachetJobScheduler
- Place config.toml in the created directory
- Create /var/log/CachetJobScheduler and ensure the user you use to run the
tool has write permissions to it
- Run CachetJobScheduler

### CachetJobScheduler arguments
- -d enables debug mode
- use -j <jobname> to only run the specified job. This also enables debug mode.
- use -lf <filepath> to specify a custom log file
