package cli

import (
	"fmt"
	"github.com/JamesArthurHolland/ozone/ozone-daemon-lib/cache"
	process_manager_client "github.com/JamesArthurHolland/ozone/ozone-daemon-lib/process-manager-client"
	"github.com/JamesArthurHolland/ozone/ozone-lib/buildables"
	ozoneConfig "github.com/JamesArthurHolland/ozone/ozone-lib/config"
	"github.com/JamesArthurHolland/ozone/ozone-lib/deployables/docker"
	"github.com/JamesArthurHolland/ozone/ozone-lib/deployables/executable"
	"github.com/JamesArthurHolland/ozone/ozone-lib/deployables/helm"
	_go "github.com/JamesArthurHolland/ozone/ozone-lib/go"
	"github.com/JamesArthurHolland/ozone/ozone-lib/utilities"
	"github.com/JamesArthurHolland/ozone/ozone-lib/utils"
	"github.com/common-nighthawk/go-figure"
	"github.com/spf13/cobra"
	"log"
	"path"
)

func init() {
	rootCmd.AddCommand(runCmd)
}

func checkCache(buildScope map[string]string) bool {
	hash, err := getBuildHash(buildScope)
	if err != nil {
		log.Fatalln(err)
		return false
	}
	if hash == "" {
		return false
	}


	serviceName := buildScope["SERVICE"]
	log.Printf("Hash is %s \n", hash)
	cachedHash := process_manager_client.CacheCheck(ozoneWorkingDir, serviceName)
	return cachedHash == hash
}

func getBuildHash(buildScope map[string]string) (string, error) {
	serviceName := buildScope["SERVICE"]
	buildName := buildScope["NAME"]
	dir := buildScope["DIR"]

	if serviceName == "" {
		log.Printf("WARNING: No servicename set on build '%s'.\n", buildName)
		return "", nil
	}
	if dir == "" {
		log.Printf("WARNING: No dir set on build '%s'.\n", buildName)
		return "", nil
	}

 	buildDirFullPath := path.Join(ozoneWorkingDir, dir)
	lastEditTime, err := cache.FileLastEdit(buildDirFullPath)

	if err != nil {
		return "", err
	}

	ozonefilePath := path.Join(ozoneWorkingDir, "Ozonefile")

	ozonefileEditTime, err := cache.FileLastEdit(ozonefilePath)

	if err != nil {
		return "", err
	}

	hash := cache.Hash(ozonefileEditTime, lastEditTime)
	return hash, nil
}

func run(builds []*ozoneConfig.Runnable, config *ozoneConfig.OzoneConfig, context string, runType ozoneConfig.RunnableType) {
	topLevelScope := ozoneConfig.CopyMap(config.BuildVars)
	topLevelScope["CONTEXT"] = context
	topLevelScope["OZONE_WORKING_DIR"] = ozoneWorkingDir

	for _, b := range builds {
		err := runIndividual(b, context, config, topLevelScope)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func runIndividual(b *ozoneConfig.Runnable, context string, config *ozoneConfig.OzoneConfig, topLevelScope map[string]string) error {
	buildScope := ozoneConfig.CopyMap(topLevelScope)
	if b.Service != "" {
		buildScope["SERVICE"] = b.Service
	}
	if b.Dir != "" {
		buildScope["DIR"] = b.Dir
	}
	buildScope["NAME"] = b.Name
	buildScope = ozoneConfig.RenderNoMerge(buildScope, topLevelScope)

	if b.Type == ozoneConfig.BuildType && checkCache(buildScope) == true {
		log.Printf("Info: build %s is cached. \n", b.Name)
		return nil
	}
	figure.NewFigure(b.Name, "doom", true).Print()

	runnableVars, err := config.FetchEnvs(b.WithEnv, buildScope)
	if err != nil {
		return err
	}

	for _, dependency := range b.Depends {
		exists, dependencyRunnable := config.FetchRunnable(dependency.Name)

		if !exists {
			log.Fatalf("Depencdency %s on build %s doesn't exist", dependency.Name, b.Name)
		}

		dependencyScope := ozoneConfig.MergeMaps(buildScope, runnableVars)
		dependencyScope = ozoneConfig.MergeMaps(dependencyScope, dependency.WithVars)
		err := runIndividual(dependencyRunnable, context, config, dependencyScope)
		if err != nil {
			return err
		}
	}

	for _, cs := range b.ContextSteps {
		match := utils.ContextInPattern(context, cs.Context)
		if match {
			contextStepVars, err := config.FetchEnvs(cs.WithEnv, buildScope)
			contextStepVars = ozoneConfig.MergeMaps(runnableVars, contextStepVars)
			contextStepBuildScope := ozoneConfig.MergeMaps(buildScope, contextStepVars)
			if err != nil {
				return err
			}
			//scope = ozoneConfig.MergeMaps(scope, runtimeVars) TODO are runtimeVarsNeeded at build?
			for _, step := range cs.Steps {

				stepVars := ozoneConfig.MergeMaps(step.WithVars, contextStepBuildScope)
				stepVars = ozoneConfig.MergeMaps(contextStepVars, stepVars)

				fmt.Printf("step %s \n", step.Name)

				if err != nil {
					return err
				}
				if step.Type == "builtin" {
					switch b.Type {
					case ozoneConfig.BuildType:
						runBuildable(step, b, stepVars)
					case ozoneConfig.DeployType:
						runDeployables(step, b, stepVars)
						//case ozoneConfig.TestTypeType:
						//	runTestables(step, b, stepVars)
					}
				}
			}
		}
	}
	// TODO update cache
	if b.Type == ozoneConfig.BuildType {
		updateCache(buildScope)
	}

	return nil
}

func updateCache(buildScope map[string]string) {
	hash, err := getBuildHash(buildScope)
	if err != nil {
		log.Fatalln(err)
	}

	serviceName := buildScope["SERVICE"]
	log.Printf("Cache updated for %s \n", serviceName)
	process_manager_client.CacheUpdate(ozoneWorkingDir, serviceName, hash)
}

func runBuildable(step *ozoneConfig.Step, r *ozoneConfig.Runnable, varsMap map[string]string) {
	switch step.Name {
	case "go":
		fmt.Println("gogo")
		err := _go.Build(
			r.Service,
			"micro-a",
			"main.go",
			varsMap,
		)
		if err != nil {
			log.Fatalln(err)
		}
	case "buildDockerImage":
		fmt.Println("Building docker image.")
		err := buildables.BuildPushDockerContainer(varsMap)
		if err != nil {
			log.Fatalln(err)
		}
	case "bashScript":
		err := utilities.RunBashScript(varsMap)
		if err != nil {
			log.Fatalln(err)
		}
	case "pushDockerImage":
		fmt.Println("Building docker image.")
		err := buildables.PushDockerImage(varsMap)
		if err != nil {
			log.Fatalln(err)
		}
	}
}

func runDeployables(step *ozoneConfig.Step, r *ozoneConfig.Runnable, varsMap map[string]string) {
	if step.Type == "builtin" {
		var err error
		switch step.Name {
		case "executable":
			fmt.Println("gogo")
			err = executable.Build(r.Service, varsMap)
			fmt.Println("after")
		case "helm":
			err = helm.Deploy(r.Service, varsMap)
		case "runDockerImage":
			err = docker.Build(varsMap)
		case "bashScript":
			err = utilities.RunBashScript(varsMap)
		default:
			log.Fatalf("Builtin value not found: %s \n", step.Name)
		}
		if err != nil {
			log.Fatalln(err)
		}
	}
}






//func deploy(deploys []*ozoneConfig.Runnable, config *ozoneConfig.OzoneConfig, context string) {
//	//varsMap := ozoneConfig.VarsToMap(config.BuildVars)
//	fmt.Println("Deploys")
//	fmt.Println(context)
//
//	for _, b := range deploys {
//		fmt.Println(b.Name)
//		fmt.Println("-")
//		for _, es := range b.ContextSteps {
//			fmt.Printf("Context: %s \n", context)
//			if es.Context == context {
//			 	buildVars := ozoneConfig.VarsToMap(config.BuildVars)
//				varsMap, err := fetchEnvs(config, es.WithEnv, buildVars)
//				varsMap = mergeMaps(buildVars, varsMap)
//				if err != nil {
//					log.Fatalln(err)
//				}
//
//				fmt.Println("Context")
//				for _, step := range es.Steps {
//					fmt.Printf("step %s", step.Type)
//					// TODO merge in step.WithVars into varsMap
//					stepVars := mergeMaps(varsMap, step.WithVars)
//				}
//			}
//		}
//	}
//}

func separateRunnables(args []string, config *ozoneConfig.OzoneConfig) ([]*ozoneConfig.Runnable,[]*ozoneConfig.Runnable,[]*ozoneConfig.Runnable) {
	var buildables []*ozoneConfig.Runnable
	var deployables []*ozoneConfig.Runnable
	var testables []*ozoneConfig.Runnable

	for _, runnableName := range args {
		if has, build := config.HasBuild(runnableName); has == true {
			buildables = append(buildables, build)
		}
		if has, deploy := config.HasDeploy(runnableName); has == true {
			deployables = append(deployables, deploy)
		}
		if has, test := config.HasTest(runnableName); has == true {
			deployables = append(testables, test)
		}
		//if isTest
	}

	return buildables, deployables, testables
}

var runCmd = &cobra.Command{
	Use:   "r",
	Long:  `List running processes`,
	Run: func(cmd *cobra.Command, args []string) {
		contextBanner := fmt.Sprintf("context::: %s", context)
		figure.NewFigure(contextBanner, "doom", true).Print()
		for _, arg := range args {
			if has, _ := config.FetchRunnable(arg); has == true {
				continue
			} else {
				log.Fatalf("Config doesn't have runnable: %s \n", arg)
			}
		}

		builds, deploys, _ := separateRunnables(args, config)

		run(builds, config, context, ozoneConfig.BuildType)
		run(deploys, config, context, ozoneConfig.DeployType)
		//tests(tests, config, context)

	},
}