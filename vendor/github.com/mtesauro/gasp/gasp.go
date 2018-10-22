// Package Gasp is a library for creating AppSec Pipelines in Golang
package gasp

import (
	"bufio"
	"fmt"
	"io"
	"io/ioutil"
	"log"
	"os"
	"os/exec"
	"path"
	"strconv"
	"time"

	"gopkg.in/yaml.v2"
)

// Interface for checking for prerequisites
type Prereq interface {
	VerifyPrereqs(d Deps)
}

type Deps struct {
	Bins          []string // Required binaries that need to be in $PATH
	Files         []string // Required files to run Gasp
	FilePath      string   // Path to required files
	ExternalFiles []string // Full path to external files (non-GASP) required by an implementation
}

// Interface for reading configs
type Config interface {
	ReadMaster(mstr *M, c ConfigOpts) // Read main configuration
	ReadSecPipe(sec *S, c ConfigOpts) // Read tools configuration
}

type ConfigOpts struct {
	Ctype string // type of config store - currently only "Local Files" supported
	Path  string // path were configs are stored
}

// Interface for listing images
type Images interface {
	SyncImages(sec *S) // Ensure needed images are available in the image repo
}

// Interface for handling event imput (command-line or otherwise)
type Event interface {
	ReadArgs(a *[]string, evArgs *EventArgs)
	GetId()
	Startup()
	Pipeline()
	Final()
	Cleanup()
}

// Struct for event arguments
type EventArgs struct {
	Profile     string            // Required - named pipeline from master.yaml, appsec.pipeline or [app name]-pipeline.yaml
	AppName     string            // Required - The name of the app the application that is the target of this pipeline run
	Target      string            // Required - The target to use for this pipeline run, generally a repo URL for SAST or URL for DAST
	DryRun      bool              // default = false - Run he pipeline without actually launching containers, basically loging only
	Keep        bool              // default = false - If true, keep any containers used during the pipeline run, default is to delete them
	Vol         string            // default = "none" - The full path to a local directory to use for all pipeline run files instead of an ephemeral data container
	Src         string            // default = "none" - The full path to a local directory which contains code for SAST pipeline runs
	Rpt         string            // default = "none" - The full path to a local directory where tool ouput/reports will be written
	AppProfile  string            // default = "none" - The application specific named pipeline (profile) to use for this run set in [app-name]-pipeline.yaml
	AppToolProf string            // default = "none" - The custom tool profile to override the default tool profile as defined in [app-name]-tool.yaml
	Loc         string            // default = "/opt/appsecpipeline/source" - Path to where the sourcecode is on the container
	ParamsRaw   string            // default = "" - Required parametetrs for the pipeline tools in this run
	ToolConf    map[string]string // calculated - map["name of tool":"string that contains parameters for tool]
	//                               e.g. ["bandit":"LOC=/opt/appsecpipeline/source"]
}

// Sturct for master.yaml aka main configuration
type M struct {
	Version    string              `yaml:"version"`
	Global     Gconf               `yaml:"global"`
	Prof       map[string]Profiles `yaml:"profiles"`
	Deployment Deploy              `yaml:"deployment"`
}

type Gconf struct {
	MinSev      string `yaml:"min-severity"`
	MaxToolRun  int    `yaml:"max-tool-run"`
	MaxParallel int    `yaml:"max-parallel"`
	MaxDynamic  int    `yaml:"max-dynamic"`
	MaxCrital   int    `yaml:"max-critical"`
	MaxHigh     int    `yaml:"max-high"`
	MaxMedium   int    `yaml:"max-medium"`
}

type Profiles struct {
	Pipeline []Tools
	Startup  []Tools
	RunEvery []Tools
	Final    []Tools
}

type Tools struct {
	Tool        string `yaml:"tool"`
	ToolProfile string `yaml:"tool-profile"`
	MinSev      string `yaml:"min-severity"`
	OnFailure   string `yaml:"on-failure"`
}

type Deploy struct {
	Master  string `yaml:"master"`
	Devel   string `yaml:"sast"`
	Preprod string `yaml:"pre-prod"`
}

// Struct for secpipeline-config.yaml aka tools configuration
type S struct {
	T map[string]SecTool `yaml:"tools"`
}

type SecTool struct {
	Version       string            `yaml:"version"`
	ToolVer       string            `yaml:"tool-version"`
	Tags          []string          `yaml:"tags"`
	ToolType      string            `yaml:"type"`
	ScanType      string            `yaml:"scan_type"`
	IconSm        string            `yaml:"icon-sm"`
	IconLg        string            `yaml:"icon-lg"`
	Description   string            `yaml:"description"`
	Url           string            `yaml:"url"`
	Documentation string            `yaml:"documentation"`
	Docker        string            `yaml:"docker"`
	Parameters    map[string]PMeta  `yaml:"parameters"`
	Cmds          map[string]string `yaml:"commands"`
	Pfls          map[string]string `yaml:"profiles"`
}

type PMeta struct {
	PType    string `yaml:"type"`
	DataType string `yaml:"data_type"`
	Desc     string `yaml:"description"`
}

// Default types for running Gasp locally aka laptop runs

// Check dependencies required for the implemented version of Gasp
type LocalDeps struct {
}

func (ld *LocalDeps) VerifyPrereqs(d Deps) {
	ready := true

	// Check for required binaries
	if len(d.Bins) > 0 {
		for k := range d.Bins {
			_, err := exec.LookPath(d.Bins[k])
			if err != nil {
				fmt.Printf("ERROR: The %s command must be installed and in your path.\n", d.Bins[k])
				fmt.Println("\tPlease install it so you can run Gasp")
				ready = false
			}
		}
	}

	// Check for required files
	if len(d.Files) > 0 {
		for k := range d.Files {
			fp := path.Join(d.FilePath, d.Files[k])
			_, err := os.Stat(fp)
			if err != nil {
				fmt.Printf("ERROR: The %s file must be exist in %s.\n", d.Files[k], d.FilePath)
				fmt.Println("\tPlease add that file so you can run Gasp")
				ready = false
			}
		}
	}

	// Check for required files
	if len(d.ExternalFiles) > 0 {
		for k := range d.ExternalFiles {
			_, err := os.Stat(d.ExternalFiles[k])
			if err != nil {
				fmt.Printf("ERROR: The %s file must exist and was not found\n", d.ExternalFiles[k])
				fmt.Println("\tPlease add that file so you can run Gasp")
				ready = false
			}
		}
	}

	if !ready {
		log.Fatal("Prerequisites to run this program missing, please correct errors above")
	}
}

// Read configuration yaml files from local disk
type LocalConfigs struct {
	ConfFile string
	ToolFile string
}

func (l *LocalConfigs) ReadMaster(mstr *M, c ConfigOpts) {
	Info.Println("Reading master.yaml file")

	fpath := path.Join(c.Path, l.ConfFile)

	f, err := ioutil.ReadFile(fpath)
	if err != nil {
		fmt.Println("Problem reading master.yaml from ", fpath)
		fmt.Printf("Error is: %s", err)
	}

	err = yaml.Unmarshal(f, mstr)
	if err != nil {
		fmt.Println("error: %v", err)
	}
}

func (l *LocalConfigs) ReadSecPipe(sec *S, c ConfigOpts) {
	fmt.Println("Reading secpipeline-conf.yaml")

	fpath := path.Join(c.Path, l.ToolFile)

	f, err := os.Open(fpath)
	if err != nil {
		fmt.Println("Problem reading secpipeline-conf.yaml from ", fpath)
		fmt.Printf("Error is: %s", err)
	}

	// Massage secpipeline-conf.yaml to account for yaml library not liking its format
	contents := "tools:\n"
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := "  " + scanner.Text()
		contents = contents + line + "\n"
	}

	err = yaml.Unmarshal([]byte(contents), sec)
	if err != nil {
		fmt.Println("Error parsing secpipeline-conf.yaml: %v", err)
	}
}

// Logging items
var (
	Trace   *log.Logger
	Info    *log.Logger
	Warning *log.Logger
	Error   *log.Logger
)

func InitLogs(traceHandle io.Writer, infoHandle io.Writer, warningHandle io.Writer, errorHandle io.Writer) map[string]*log.Logger {

	Trace = log.New(traceHandle, "TRACE:   ", log.Ldate|log.Ltime|log.Lshortfile)
	Info = log.New(infoHandle, "INFO:    ", log.Ldate|log.Ltime)
	Warning = log.New(warningHandle, "WARNING: ", log.Ldate|log.Ltime)
	Error = log.New(errorHandle, "ERROR:   ", log.Ldate|log.Ltime|log.Lshortfile)

	// Allows for these to be use globally
	// Trace.Println("Trace message")
	// Info.Println("Info message")
	// Warning.Println("Warning message")
	// Error.Println("Error message")

	return map[string]*log.Logger{
		"trace": Trace,
		"info":  Info,
		"warn":  Warning,
		"error": Error,
	}
}

func SetupLogging(name string, logPath string, timestamp bool) map[string]*log.Logger {
	// Setup logging
	when := ""
	if timestamp {
		n := time.Now()
		when = "_" + strconv.Itoa(int(n.UnixNano()))
	}

	logName := name + when + ".log"
	fullPath := path.Join(logPath, logName)
	logFile, err := os.OpenFile(fullPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		fmt.Printf("\nPlease create any directories needed to write logs to %v\n\n", logPath)
		log.Fatalf("Failed to open log file %s.  Error was:\n  %+v\n", logPath, err)
	}
	// Log everthing to the specificied log file location
	return InitLogs(logFile, logFile, logFile, logFile)
}
