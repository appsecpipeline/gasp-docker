// gasp.go
package main

import (
	"bufio"
	"bytes"
	"fmt"
	"io"
	"log"
	"os"
	"os/exec"
	"path"
	"strings"

	g "github.com/mtesauro/gasp"
	"github.com/satori/go.uuid"
)

// Interfaces from Gasp for this implementation

// Implementation of the Images interface from Gasp
type LocalDockers struct {
	detailed   *os.File
	runId      string
	resultsVol string
	images     []Image
}

type Image struct {
	fullName string
	name     string
	tag      string
	id       string
	created  string
	size     string
}

func (ldock *LocalDockers) SyncImages(sec *g.S) {
	infoLog.Println("Ensuring needed tool images are available in the container repo")

	// Get a list of images available in the repo
	repoImages := listImages(ldock)

	// Diff needed images against available ones
	missing := diffImages(repoImages, sec)

	// Pull any needed images so all tools containers are available
	pullImages(missing, ldock.detailed)
	infoLog.Println("All needed tool images are available in the container repo")
}

// Implementation of the Event interface from gasp
type LocalEvent struct {
}

func (levent *LocalEvent) ReadArgs(args *[]string, evArgs *g.EventArgs) {
	fmt.Println("In ReadArgs function")

	evArgs.Required = []string{"Profile", "PipeType"}
	evArgs.Profile = "debug-test"
	evArgs.Dir = ""
	evArgs.DryRun = false
	evArgs.Clean = true
	evArgs.Vol = "/home/mtesauro/Dropbox/projects/pipeline-python-example"
	evArgs.AppName = "Trusted Test"
	evArgs.EventType = "command-line"
	evArgs.AppProfile = ""
	evArgs.AppToolProf = ""
	evArgs.Target = ""
	evArgs.PipeType = "static"
	evArgs.Loc = "/opt/appsecpipeline/source"
	evArgs.DojoHost = "dojohost.example.pvt"
	evArgs.DojoApiKey = "admin:ad38adc0d33e6b0d9c452acec891e5ca413480e7"
	evArgs.DojoProdId = "2"

	// Setup tool configuration
	evArgs.ToolConf = map[string]string{
		"git":        "-w http://10.214.117.100:9000/hooks/appsecpipeline GIT_URL=https://github.com/aaronweaver/AppSecPipeline.git LOC=/opt/appsecpipeline/source",
		"bandit":     "-w http://10.214.117.100:9000/hooks/appsecpipeline LOC=/opt/appsecpipeline/source",
		"defectdojo": "-w http://10.214.117.100:9000/hooks/appsecpipeline ",
	}

	//TODO Handle changes below in the hard-coded junk above => See specification

	// Struct for event arguments
	//type EventArgs struct {
	//	Required    []string // names of required fields will be in this slice of strings
	//	Profile     string   // Required - named pipeline from master.yaml, appsec.pipeline or [app name]-pipeline.yaml
	//	Dir         string   // default to empty string - local directory to copy to Docke shared volume
	//	DryRun      bool     // default = false - Do this pipeline run without launching actual dockers, etc
	//	Persist     bool     // default = false - Keep the containers and volumes once completed
	//	Vol         string   // default = "" - Specify the name of the results volumne to use
	//     => handle by convention {UUID}_results
	//	AppName     string   // default = "" - Name of the app being tested
	//	EventType   string   // default = "Command-line" - Type of event that caused the pipeline run
	//	AppProfile  string   // default = "" - App specific pipeline profile to run sent in [app-name]-pipeline.yaml
	//	AppToolProf string   // default = "" - App specific tool profile to use during pipeline run from [app-name]-tool.yaml
	//	Target      string   // default = "" - Docker name of the target for the pipeline run - to be launched before run
	//	PipeType    string   // Required - {"static", "dynamic"}
	//	Loc         string   // default = "" - Location of source code for static test runs
	//    => handle by convention /opt/appsecpipeline/source
	//	DojoHost    string   // Required - host name of the Dojo instance to push the run restults to
	//	DojoApiKey  string   // Required - API key to talk to Dojo's REST API
	//	DojoProdId  string   // Required - The Product ID from Dojo to submit the results for this test run
	//	//DojoNewEng boot // default = true - Create a new engagement for each pipeline run?
	//}
}

func (levent *LocalEvent) GetId() string {
	// Generate a type 4 UUID to use as an identifier of this run
	u, err := uuid.NewV4()
	if err != nil {
		errorLog.Printf("Error generating run ID, errror was: %s", err)
		errorLog.Println("Unable to get a unique ID for this run, quitting")
		os.Exit(1)
	}

	return u.String()
}

func (levent *LocalEvent) Startup(ld *LocalDockers, mstr *g.M, sec *g.S, ev *g.EventArgs) {
	// Handle any defined startup tool runs for this named pipeline
	fmt.Println("In Startup function")

	// Create a results volume which may also be used for source code for SAST
	fmt.Println("Calling launchVolume")
	vol, err := launchVolume(ld)
	if err != nil {
		panic(err)
		// TODO - take error from launchVolume
	}
	fmt.Println("Created volume named ", vol)

	// Set results volume for this run
	ld.resultsVol = vol

	// Adjust the file permission of the volume to use the appsecpipeline user
	err = volumePerms(vol, ld)
	if err != nil {
		panic(err)
		// TODO - take error from volumePerms
	}

	// Interate over the defined startup steps, running them in order
	for _, v := range mstr.Prof[ev.Profile].Startup {
		err := launchContainer(v, vol, ev, ld, sec)
		if err != nil {
			panic(err)
			// TODO - take error from launchVolume
		}
	}

}

func (levent *LocalEvent) Pipeline(ld *LocalDockers, mstr *g.M, sec *g.S, ev *g.EventArgs) {
	fmt.Println("In Pipeline function")

	// Interate over the defined pipeline steps, running them in order
	for _, v := range mstr.Prof[ev.Profile].Pipeline {
		err := launchContainer(v, ld.resultsVol, ev, ld, sec)
		if err != nil {
			panic(err)
			// TODO - take error from launchVolume
		}
	}
}

func (levent *LocalEvent) Final() {
	fmt.Println("In Final function")
}

func (levent *LocalEvent) Cleanup() {
	fmt.Println("In Cleanup function")
}

// Vars and functions for gasp-docker
var logDir string = "./logs"
var traceLog *log.Logger
var infoLog *log.Logger
var warnLog *log.Logger
var errorLog *log.Logger

func listImages(ldock *LocalDockers) []Image {
	infoLog.Println("Getting list of Docker images available in repo")

	cmd := exec.Command("docker", "images")
	var sOut, sErr bytes.Buffer
	cmd.Stdout = &sOut
	cmd.Stderr = &sErr
	err := cmd.Run()
	if err != nil {
		errorLog.Printf("Error getting image list, errror was: %s", sOut.String())
		errorLog.Println("Unable to get a list of available Docker images, quitting")
		os.Exit(1)
	}
	io.Copy(ldock.detailed, bytes.NewReader(sOut.Bytes()))

	scanner := bufio.NewScanner(bytes.NewReader(sOut.Bytes()))
	images := make([]Image, 25)
	for scanner.Scan() {
		line := scanner.Text()
		// Strip off header line, process the rest
		if !strings.Contains(line, "REPOSITORY") {
			bits := strings.Fields(line)
			fn := bits[0] + ":" + bits[1]
			im := Image{
				fullName: fn,
				name:     bits[0],
				tag:      bits[1],
				id:       bits[2],
				created:  bits[3],
				size:     bits[4],
			}
			images = append(images, im)
		}
	}
	return images
}

func diffImages(r []Image, s *g.S) map[string]bool {
	infoLog.Println("Diff'ing needed against available Docker images")

	// Get a list of needed images with no duplicates, make all false aka not available
	inRepo := make(map[string]bool)
	for _, val := range s.T {
		inRepo[val.Docker] = false
	}

	// Compare needed against available in the repo
	for _, img := range r {
		// If the map has a key that's the name of an existing image, set to true aka available
		if _, ok := inRepo[img.fullName]; ok {
			inRepo[img.fullName] = true
			infoLog.Printf("Image %s is available on this system, no need to pull", img.fullName)
		}
	}
	return inRepo
}

func pullImages(i map[string]bool, detailed *os.File) {
	infoLog.Println("Pulling any needed images")

	// Run through missing list and pull images as needed
	c := 0
	for k, v := range i {

		if !v {
			// false aka need to pull this image
			infoLog.Printf("Image needed, pulling image %s\n", k)
			infoLog.Println("This will take a bit depending on network speeds")
			cmd := exec.Command("docker", "pull", k)
			var sOut, sErr bytes.Buffer
			cmd.Stdout = &sOut
			cmd.Stderr = &sErr
			err := cmd.Run()
			if err != nil {
				errorLog.Printf("Error pulling image %s, errror was: %s", k, sOut.String())
				errorLog.Println("Unable to pull required Docker image, quitting")
				os.Exit(1)
			}
			io.Copy(detailed, bytes.NewReader(sOut.Bytes()))
			infoLog.Printf("Success pulling image %s\n", k)
			c += 1
		}
	}

	if c == 0 {
		infoLog.Println("No images to pull")
	}
	infoLog.Println("Completed pulling needed images")
}

func launchVolume(ld *LocalDockers) (string, error) {
	// Create a data volume to use for tools results and possibly SAST targets
	vname := "data_" + ld.runId

	cmd := exec.Command("docker", "volume", "create", vname)
	var sOut, sErr bytes.Buffer
	cmd.Stdout = &sOut
	cmd.Stderr = &sErr
	err := cmd.Run()
	if err != nil {
		errorLog.Printf("Error creating data volume %s, errror was: %s", vname, sOut.String())
		errorLog.Println("Unable to create data volume, quitting")
		os.Exit(1)
	}
	io.Copy(ld.detailed, bytes.NewReader(sOut.Bytes()))
	infoLog.Printf("Success creating data volume %s\n", vname)

	return vname, nil
}

func volumePerms(vol string, ld *LocalDockers) error {
	// Ajust the file permissions of the new volume so they are owned by the appsecpipeline user
	volMount := vol + ":/opt/appsecpipeline/"
	dName := "set-perms_" + ld.runId

	cmd := exec.Command("docker", "run", "-v", volMount, "--name", dName,
		"--user=root", "--entrypoint", "chown", "appsecpipeline/base:1.4",
		"-R", "appsecpipeline:appsecpipeline", "/opt/appsecpipeline")
	var sOut, sErr bytes.Buffer
	cmd.Stdout = &sOut
	cmd.Stderr = &sErr
	err := cmd.Run()
	if err != nil {
		errorLog.Printf("Error setting file permissions on data volume %s, errror was: %s\n%s", vol, sErr.String(), err)
		errorLog.Println("Unable to create data volume with needed file permissions, quitting")
		os.Exit(1)
	}
	io.Copy(ld.detailed, bytes.NewReader(sOut.Bytes()))
	infoLog.Printf("Successfully set file permissions on data volume %s\n", vol)

	return nil

}

func launchContainer(tool g.Tools, vol string, ev *g.EventArgs, ld *LocalDockers, sec *g.S) error {
	// Run the provided tool from this portion of the named pipeline run
	dName := tool.Tool + "_" + ld.runId
	volMount := vol + ":/opt/appsecpipeline/"

	// Get the docker image for this tool
	fmt.Printf("docker image is %v\n", sec.T[tool.Tool].Docker)

	// Gather the needed parameters for this tool run
	toolArgs := strings.Split(ev.ToolConf[tool.Tool], " ")
	fmt.Printf("Tool Args now looks like %v\n", toolArgs)

	//fmt.Printf("The tool to run is %v of type %T\n", tool, tool)
	fmt.Printf("Time to run tool called %s with %s option selected\n", tool.Tool, tool.Options)

	// build the command for this tool
	args := []string{
		"run",
		"-v", volMount,
		"--net=host",
		"--name", dName,
		sec.T[tool.Tool].Docker, // Docker image for this tool
		"-t",
		tool.Tool,
		"-p",
		tool.Options,
	}

	// Add on tool specific arguments
	args = append(args, toolArgs...)

	// Run the container
	cmd := exec.Command("docker", args...)
	var sOut, sErr bytes.Buffer
	cmd.Stdout = &sOut
	cmd.Stderr = &sErr
	err := cmd.Run()
	if err != nil {
		errorLog.Printf("Error launching container %s, errror was: %s\n%s\n%s", dName, sErr.String(), err, sOut.String())
		errorLog.Println("Unable to launch container for pipeline stage, quitting")
		os.Exit(1)
	}
	io.Copy(ld.detailed, bytes.NewReader(sOut.Bytes()))
	infoLog.Printf("Successfully launched container %s\n", dName)

	return nil
}

func main() {
	// Start gasp-docker logging
	l := g.SetupLogging("gasp-docker", logDir, true)
	traceLog = l["trace"]
	infoLog = l["info"]
	warnLog = l["warn"]
	errorLog = l["error"]
	infoLog.Println("Logging setup for gasp-docker")

	// Startup detailed logging
	ldock := LocalDockers{}
	fullPath := path.Join(logDir, "detailed.log")
	dL, err := os.Create(fullPath)
	if err != nil {
		fmt.Printf("\nPlease create any directories needed to write logs to %v\n\n", logDir)
		log.Fatalf("Failed to open log file %s.  Error was:\n  %+v\n", fullPath, err)
	}
	ldock.detailed = dL
	defer dL.Close()

	// Check Dependencies
	d := g.Deps{
		[]string{"docker"},
		[]string{"master.yaml", "secpipeline-config.yaml"},
		"./controller",
		[]string{},
	}
	ld := g.LocalDeps{}
	ld.VerifyPrereqs(d)
	infoLog.Println("All dependencies needed for gasp-docker are available")

	// Read the configs to set things up
	lconf := g.LocalConfigs{"master.yaml", "secpipeline-config.yaml"}
	mstr := g.M{}
	lopts := g.ConfigOpts{"Local Files", "./controller"}
	lconf.ReadMaster(&mstr, lopts)
	sec := g.S{}
	lconf.ReadSecPipe(&sec, lopts)

	// Sync images so all needed tool images are in image repo
	ldock.SyncImages(&sec)

	// handleEvent
	eArgs := g.EventArgs{}
	le := LocalEvent{}
	le.ReadArgs(&os.Args, &eArgs)

	// Run the sent Named Profile
	ldock.runId = le.GetId()

	fmt.Printf("Run id is %s\n", ldock.runId)

	// startup
	le.Startup(&ldock, &mstr, &sec, &eArgs)

	// pipeline
	le.Pipeline(&ldock, &mstr, &sec, &eArgs)

	// final

	// cleanup

	//TODO: Add more meta to the detailed log

	// DEBUG INFO
	fmt.Println("\nDEBUG INFO\n")

	fmt.Println("Length of SecPipeline is:", len(sec.T))
	//fmt.Printf("What is this value: %v\n", sec.T)
	//fmt.Println("Configured tools are:")
	//	for _, val := range sec.T {
	//		//fmt.Println("key is %v", key)
	//		//fmt.Println("val is %v", val)
	//		//fmt.Println("val is %T", val)
	//		fmt.Printf("\t%v\n", val.Docker)
	//	}

	fmt.Println("###############################")
	fmt.Printf("The named pipline '%s' consists of:\n", eArgs.Profile)
	fmt.Printf("Startup contains:\n%v\n", mstr.Prof["debug-test"].Startup)
	fmt.Printf("Pipeline contains:\n%v\n", mstr.Prof["debug-test"].Pipeline)
	fmt.Printf("Final contains:\n%v\n", mstr.Prof["debug-test"].Final)

	fmt.Println("\n###############################")
	fmt.Println("Copy/Paste cleanup\n")
	fmt.Printf("docker rm set-perms_%s\n", ldock.runId)
	fmt.Printf("docker rm git_%s\n", ldock.runId)
	fmt.Printf("docker rm bandit_%s\n", ldock.runId)
	fmt.Printf("docker volume rm data_%s\n\n", ldock.runId)

}
