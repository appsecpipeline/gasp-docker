# gasp-docker

Simple implementation of an AppSec Pipeline using the Gasp library

It's designed to be a useful tool and an example of the AppSec Pipeline specification in action.

The uber-short description of gasp-docker is it provides a way to do app testing automation by running 1 or more Docker-ified security tool against an application or its source code.

### Getting to know gasp-docker

gasp-docker uses commands similar to Docker with base commands and arguements for those base commands.  Currently, the implemented commands are run and version.  Version prints the version of gasp-docker and run is the primary command used to conduct AppSec Pipeline runs.  The available commands are displayed with the --help argument:

```
$ ./gasp-docker --help
gasp-docker is a Golang implementation of the AppSec Pipeline spec 
using Docker running locally.  You must have Docker installed for 
this program to work. 

Usage for gasp-docker:  gasp-docker COMMAND

Usage:
  gasp-docker [command]

Available Commands:
  config      A brief description of your command
  help        Help about any command
  list        A brief description of your command
  run         Run the provided named pipeline aka profile from master.yaml
  version     A brief description of your command

Flags:
  -h, --help   help for gasp-docker

Use "gasp-docker [command] --help" for more information about a command.
```

So, since run the the main command you'll use, let's see what options are available for that:

```
./gasp-docker run --help
Run the provide named pipeline aka profile from master.yaml

For example:
  gasp-docker run -profile="pre-launch"

would run the pipeline called "pre-launch" as defined in master.yaml

Usage:
  gasp-docker run [flags]

Flags:
  -a, --app-name string       <required> The name of the app the application that is the target of this pipeline run
  -f, --app-profile string    The application specific named pipeline (profile) to use for this run in [app-name]-pipeline.yaml (default "none")
  -d, --dry-run               If present, run he pipeline without actually launching containers, basically loging only
  -h, --help                  help for run
  -k, --keep                  If present, keep any containers used during the pipeline run
  -l, --location string       Path to where the sourcecode is in the container (default "/opt/appsecpipeline/source")
  -m, --params string         Required parametetrs for the pipeline tools in this run
  -p, --profile string        <required> The named pipeline aka profile from master.yaml to run
  -r, --reports string        The full path to a local directory which contains source code for SAST pipeline runs (default "none")
  -s, --source string         The full path to a local directory to use for /opt/appsecpipeline/reports (default "none")
  -t, --target string         The target to use for this pipeline run, generally a repo URL for SAST or URL for DAST (default "TBD")
  -o, --tool-profile string   The custom tool profile to override the profiles defined in secpipeline-config.yaml for this run (default "none")
  -v, --volume string         The full path to a local directory to use for all pipeline run files instead of an ephemeral data container (default "none")
```

Let's look at those options is in a bit more detail. First, the mandatory ones:

-a, --app-name string

* *The name of the app the application that is the target of this pipeline run*
* This string is primarily used to do human friendly logging

-m, --params string

* *Required parameters for the pipeline tools in this run*
* Depending on the tools in your profile, 1+ parameters will be required put all of them in a single string like -m “Param1=value Param2=value2”

-p, --profile string

* *The named pipeline aka profile from master.yaml to run*
* The name of the list of tools to run in the order they are listed in master.yaml aka the pipeline configuration for this run

Now the optional commands:

-f, --app-profile string

* *The application specific named pipeline (profile) to use for this run in [app-name]-pipeline.yaml (default "none")*
* Allows overriding defined named pipelines for ad-hoc/custom runs

-d, --dry-run

* *If present, run he pipeline without actually launching containers, basically logging only*
* A way to test a run to ensure all config, etc is available.

-k, --keep

* *If present, keep any containers used during the pipeline run*
* Don’t remove Docker images after a run, mostly for debugging

-l, --location string

* *Path to where the source code is in the container (default "/opt/appsecpipeline/source")*
* If you need to override the location of source code in the container

-r, --reports string

* *The full path to a local directory to use for /opt/appsecpipeline/reports (default "none")*
* Allows you to override the location that reports are written to a local directory

-s, --source string

* *The full path to a local directory which contains source code for SAST pipeline runs (default "none")*
* Allows you to override the location for source code to a local directory

-t, --target string

* *The target to use for this pipeline run, generally a repo URL for SAST or URL for DAST (default "TBD")*
* Hasn’t been implemented.  Idea was to take a tools parameters and store them to be accessed by the target string

-o, --tool-profile string

* *The custom tool profile to override the profiles defined in secpipeline-config.yaml for this run (default "none")*
* Allows you to run an custom/ad-hoc tool profile

-v, --volume string

* *The full path to a local directory to use for all pipeline run files instead of an ephemeral data container (default "none")*

gasp-docker will read 2 files in the ‘spec’ sub-directory where it’s run.  These are the master.yaml and secpipeline-config.yaml files.  The files have two distinct roles to play with how gasp-docker runs.

**secpipeline-config.yaml** lists all the tools that are available to use when creating a named pipeline (a specific combination of tools in a specific order)  gasp-docker uses this file to determine

* What docker to use for a specific tool
* What parameters are supported by the tool
* What command(s) to run to execute the tool and produce a report file from the execution
* What profiles are available for the tool.  Profiles provide different ways to run a tool based on available options (light test, thorough test, etc)

**master.yaml** provides some global configuration items plus a collection of named pipelines.

* Named pipelines are a collection of tools run in a specific order. Named pipelines can have 3 stages
* * Startup (this stage is run first)
* * Pipeline (this stage is run second and the only mandatory stage)
* * Final (this stage is run last)
* Each stage can contain 1 or more tool and a tool profile to run that tool under
* The smallest possible named pipeline would be a Pipeline stage with only 1 tool defined.

A presentation that provides an overview of OWASP's AppSec Pipeline projects can be found on [slideshare](https://www.slideshare.net/mtesauro/making-continuous-security-a-reality-with-owasps-appsec-pipeline-matt-tesauro-aaron-weaver) or OWASP's [YouTube Channel](https://www.youtube.com/watch?v=UCwkAQXN6TE&index=31&list=PLpr-xdpM8wG9yT6HD6YeCbf6wymhAAqRb&t=0s)