// Copyright 2018 The Kubeflow Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//       http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package commands

import (
	"fmt"
	"strings"

	"github.com/kubeflow/arena/pkg/util"
	"github.com/kubeflow/arena/pkg/util/helm"
	"github.com/kubeflow/arena/pkg/workflow"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"github.com/spf13/viper"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var (
	// to receive values from command operation --worker-selector
	workerSelectors []string
	// to receive values from command operation --evaluator-selector
	evaluatorSelectors []string
	// to receive values from command operation --ps-selector
	psSelectors []string
	// to receive values from command operation --chief-selector
	chiefSelectors []string
	tfjob_chart    = util.GetChartsFolder() + "/tfjob"
)

func NewSubmitTFJobCommand() *cobra.Command {
	var (
		submitArgs submitTFJobArgs
	)

	submitArgs.Mode = "tfjob"

	var command = &cobra.Command{
		Use:     "tfjob",
		Short:   "Submit TFJob as training job.",
		Aliases: []string{"tf"},
		PreRun: func(cmd *cobra.Command, args []string) {
			viper.BindPFlags(cmd.Flags())
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) == 0 {
				cmd.HelpFunc()(cmd, args)
				return fmt.Errorf("not found command args")
			}

			util.SetLogLevel(logLevel)
			setupKubeconfig()
			_, err := initKubeClient()
			if err != nil {
				log.Errorf("Failed due to %v", err)
				return err
			}

			err = updateNamespace(cmd)
			if err != nil {
				log.Errorf("Failed due to %v", err)
				return err
			}

			err = submitTFJob(args, &submitArgs)
			if err != nil {
				log.Errorf("Failed due to %v", err)
				return err
			}
			return nil
		},
	}

	submitArgs.addCommonFlags(command)
	submitArgs.addSyncFlags(command)

	// TFJob
	command.Flags().StringVar(&submitArgs.WorkerImage, "workerImage", "", "the docker image for tensorflow workers")
	command.Flags().MarkDeprecated("workerImage", "please use --worker-image instead")
	command.Flags().StringVar(&submitArgs.WorkerImage, "worker-image", "", "the docker image for tensorflow workers")

	command.Flags().StringVar(&submitArgs.PSImage, "psImage", "", "the docker image for tensorflow workers")
	command.Flags().MarkDeprecated("psImage", "please use --ps-image instead")
	command.Flags().StringVar(&submitArgs.PSImage, "ps-image", "", "the docker image for tensorflow workers")

	command.Flags().IntVar(&submitArgs.PSCount, "ps", 0, "the number of the parameter servers.")

	command.Flags().IntVar(&submitArgs.PSPort, "psPort", 0, "the port of the parameter server.")
	command.Flags().MarkDeprecated("psPort", "please use --ps-port instead")
	command.Flags().IntVar(&submitArgs.PSPort, "ps-port", 0, "the port of the parameter server.")

	command.Flags().IntVar(&submitArgs.WorkerPort, "workerPort", 0, "the port of the worker.")
	command.Flags().MarkDeprecated("workerPort", "please use --worker-port instead")
	command.Flags().IntVar(&submitArgs.WorkerPort, "worker-port", 0, "the port of the worker.")

	command.Flags().StringVar(&submitArgs.WorkerCpu, "workerCpu", "", "the cpu resource to use for the worker, like 1 for 1 core.")
	command.Flags().MarkDeprecated("workerCpu", "please use --worker-cpu instead")
	command.Flags().StringVar(&submitArgs.WorkerCpu, "worker-cpu", "", "the cpu resource to use for the worker, like 1 for 1 core.")

	command.Flags().StringVar(&submitArgs.WorkerMemory, "workerMemory", "", "the memory resource to use for the worker, like 1Gi.")
	command.Flags().MarkDeprecated("workerMemory", "please use --worker-memory instead")
	command.Flags().StringVar(&submitArgs.WorkerMemory, "worker-memory", "", "the memory resource to use for the worker, like 1Gi.")

	command.Flags().StringVar(&submitArgs.PSCpu, "psCpu", "", "the cpu resource to use for the parameter servers, like 1 for 1 core.")
	command.Flags().MarkDeprecated("psCpu", "please use --ps-cpu instead")
	command.Flags().StringVar(&submitArgs.PSCpu, "ps-cpu", "", "the cpu resource to use for the parameter servers, like 1 for 1 core.")

	command.Flags().IntVar(&submitArgs.PSGpu, "ps-gpus", 0, "the gpu resource to use for the parameter servers, like 1 for 1 gpu.")

	command.Flags().StringVar(&submitArgs.PSMemory, "psMemory", "", "the memory resource to use for the parameter servers, like 1Gi.")
	command.Flags().MarkDeprecated("psMemory", "please use --ps-memory instead")
	command.Flags().StringVar(&submitArgs.PSMemory, "ps-memory", "", "the memory resource to use for the parameter servers, like 1Gi.")
	command.Flags().StringArrayVarP(&psSelectors, "ps-selector", "", []string{}, `assigning jobs with "PS" role to some k8s particular nodes(this option would cover --selector), usage: "--ps-selector=key=value"`)
	// How to clean up Task
	command.Flags().StringVar(&submitArgs.CleanPodPolicy, "cleanTaskPolicy", "Running", "How to clean tasks after Training is done, only support Running, None.")
	command.Flags().MarkDeprecated("cleanTaskPolicy", "please use --clean-task-policy instead")
	command.Flags().StringVar(&submitArgs.CleanPodPolicy, "clean-task-policy", "Running", "How to clean tasks after Training is done, only support Running, None.")

	// Tensorboard
	command.Flags().BoolVar(&submitArgs.UseTensorboard, "tensorboard", false, "enable tensorboard")
	command.Flags().StringVar(&submitArgs.TensorboardImage, "tensorboardImage", "registry.cn-zhangjiakou.aliyuncs.com/tensorflow-samples/tensorflow:1.12.0-devel", "the docker image for tensorboard")
	command.Flags().MarkDeprecated("tensorboardImage", "please use --tensorboard-image instead")
	command.Flags().StringVar(&submitArgs.TensorboardImage, "tensorboard-image", "registry.cn-zhangjiakou.aliyuncs.com/tensorflow-samples/tensorflow:1.12.0-devel", "the docker image for tensorboard")

	command.Flags().StringVar(&submitArgs.TrainingLogdir, "logdir", "/training_logs", "the training logs dir, default is /training_logs")

	// Estimator
	command.Flags().BoolVar(&submitArgs.UseChief, "chief", false, "enable chief, which is required for estimator.")
	command.Flags().BoolVar(&submitArgs.UseEvaluator, "evaluator", false, "enable evaluator, which is optional for estimator.")
	command.Flags().StringVar(&submitArgs.ChiefCpu, "ChiefCpu", "", "the cpu resource to use for the Chief, like 1 for 1 core.")
	command.Flags().MarkDeprecated("ChiefCpu", "please use --chief-cpu instead")
	command.Flags().StringVar(&submitArgs.ChiefCpu, "chief-cpu", "", "the cpu resource to use for the Chief, like 1 for 1 core.")

	command.Flags().StringVar(&submitArgs.ChiefMemory, "ChiefMemory", "", "the memory resource to use for the Chief, like 1Gi.")
	command.Flags().MarkDeprecated("ChiefMemory", "please use --chief-memory instead")
	command.Flags().StringVar(&submitArgs.ChiefMemory, "chief-memory", "", "the memory resource to use for the Chief, like 1Gi.")

	command.Flags().StringVar(&submitArgs.EvaluatorCpu, "evaluatorCpu", "", "the cpu resource to use for the evaluator, like 1 for 1 core.")
	command.Flags().MarkDeprecated("evaluatorCpu", "please use --evaluator-cpu instead")
	command.Flags().StringVar(&submitArgs.EvaluatorCpu, "evaluator-cpu", "", "the cpu resource to use for the evaluator, like 1 for 1 core.")

	command.Flags().StringVar(&submitArgs.EvaluatorMemory, "evaluatorMemory", "", "the memory resource to use for the evaluator, like 1Gi.")
	command.Flags().MarkDeprecated("evaluatorMemory", "please use --evaluator-memory instead")
	command.Flags().StringVar(&submitArgs.EvaluatorMemory, "evaluator-memory", "", "the memory resource to use for the evaluator, like 1Gi.")

	command.Flags().IntVar(&submitArgs.ChiefPort, "chiefPort", 0, "the port of the chief.")
	command.Flags().MarkDeprecated("chiefPort", "please use --chief-port instead")
	command.Flags().IntVar(&submitArgs.ChiefPort, "chief-port", 0, "the port of the chief.")
	command.Flags().StringArrayVarP(&workerSelectors, "worker-selector", "", []string{}, `assigning jobs with "Worker" role to some k8s particular nodes(this option would cover --selector), usage: "--worker-selector=key=value"`)
	command.Flags().StringArrayVarP(&chiefSelectors, "chief-selector", "", []string{}, `assigning jobs with "Chief" role to some k8s particular nodes(this option would cover --selector), usage: "--chief-selector=key=value"`)
	command.Flags().StringArrayVarP(&evaluatorSelectors, "evaluator-selector", "", []string{}, `assigning jobs with "Evaluator" role to some k8s particular nodes(this option would cover --selector), usage: "--evaluator-selector=key=value"`)

	// command.Flags().BoolVarP(&showDetails, "details", "d", false, "Display details")
	return command
}

type submitTFJobArgs struct {
	TFNodeSelectors map[string]map[string]string `yaml:"tfNodeSelectors"`
	Port            int                          // --port, it's used set workerPort and PSPort if they are not set
	WorkerImage     string                       `yaml:"workerImage"` // --workerImage
	WorkerPort      int                          `yaml:"workerPort"`  // --workerPort
	PSPort          int                          `yaml:"psPort"`      // --psPort
	//PSNodeSelectors map[string]string `yaml:"psNodeSelectors"` // --ps-selector
	PSCount   int    `yaml:"ps"`        // --ps
	PSImage   string `yaml:"psImage"`   // --psImage
	WorkerCpu string `yaml:"workerCPU"` // --workerCpu
	//WorkerNodeSelectors map[string]string `yaml:"workerNodeSelectors"` // --worker-selector
	WorkerMemory   string `yaml:"workerMemory"`   // --workerMemory
	PSCpu          string `yaml:"psCPU"`          // --psCpu
	PSGpu          int    `yaml:"psGPU"`          // --ps-gpus
	PSMemory       string `yaml:"psMemory"`       // --psMemory
	CleanPodPolicy string `yaml:"cleanPodPolicy"` // --cleanTaskPolicy
	// For esitmator, it reuses workerImage
	UseChief     bool `yaml:",omitempty"` // --chief
	ChiefCount   int  `yaml:"chief"`
	UseEvaluator bool `yaml:",omitempty"` // --evaluator
	ChiefPort    int  `yaml:"chiefPort"`  // --chiefPort
	//ChiefNodeSelectors map[string]string `yaml:"chiefNodeSelectors"` // --chief-selector
	ChiefCpu     string `yaml:"chiefCPU"`     // --chiefCpu
	ChiefMemory  string `yaml:"chiefMemory"`  // --chiefMemory
	EvaluatorCpu string `yaml:"evaluatorCPU"` // --evaluatorCpu
	//EvaluatorNodeSelectors map[string]string `yaml:"evaluatorNodeSelectors"` // --evaluator-selector
	EvaluatorMemory string `yaml:"evaluatorMemory"` // --evaluatorMemory
	EvaluatorCount  int    `yaml:"evaluator"`

	// determine if it has gang scheduler
	HasGangScheduler bool `yaml:"hasGangScheduler"`

	// for common args
	submitArgs `yaml:",inline"`

	// for tensorboard
	submitTensorboardArgs `yaml:",inline"`

	// for sync up source code
	submitSyncCodeArgs `yaml:",inline"`

	tfRuntime `yaml:"-"`
}

func (submitArgs *submitTFJobArgs) prepare(args []string) (err error) {
	submitArgs.Command = strings.Join(args, " ")

	err = submitArgs.transform()
	if err != nil {
		return err
	}

	// 1. Use specified runtime to transform
	if submitArgs.tfRuntime != nil {
		err := submitArgs.tfRuntime.transform(submitArgs)
		if err != nil {
			return err
		}
	}

	err = submitArgs.check()
	if err != nil {
		return err
	}

	// 2. Use specified runtime to check
	if submitArgs.tfRuntime != nil {
		err := submitArgs.tfRuntime.check(submitArgs)
		if err != nil {
			return err
		}
	}

	err = submitArgs.HandleSyncCode()
	if err != nil {
		return err
	}

	commonArgs := &submitArgs.submitArgs
	err = commonArgs.transform()
	if err != nil {
		return nil
	}
	if err := submitArgs.addConfigFiles(); err != nil {
		return err
	}
	// process tensorboard
	submitArgs.processTensorboard(submitArgs.DataSet)

	if len(envs) > 0 {
		submitArgs.Envs = transformSliceToMap(envs, "=")
	}

	submitArgs.processCommonFlags()

	submitArgs.addTFNodeSelectors()

	return nil
}

func (submitArgs submitTFJobArgs) check() error {
	err := submitArgs.submitArgs.check()
	if err != nil {
		return err
	}

	switch submitArgs.CleanPodPolicy {
	case "None", "Running":
		log.Debugf("Supported cleanTaskPolicy: %s", submitArgs.CleanPodPolicy)
	default:
		return fmt.Errorf("Unsupported cleanTaskPolicy %s", submitArgs.CleanPodPolicy)
	}

	if submitArgs.WorkerCount == 0 && !submitArgs.UseChief {
		return fmt.Errorf("--workers must be greater than 0 in distributed training")
	}

	if submitArgs.WorkerImage == "" {
		return fmt.Errorf("--image or --workerImage must be set")
	}

	if submitArgs.PSCount > 0 {
		if submitArgs.PSImage == "" {
			return fmt.Errorf("--image or --psImage must be set")
		}
	}

	return nil
}

// This method for supporting tf-estimator
func (submitArgs *submitTFJobArgs) setStandaloneMode() {
	if submitArgs.PSCount < 1 && submitArgs.WorkerCount == 1 {
		submitArgs.UseChief = true
		submitArgs.WorkerCount = 0
	}
}

func (submitArgs *submitTFJobArgs) transform() error {

	submitArgs.setStandaloneMode()

	if submitArgs.WorkerImage == "" {
		submitArgs.WorkerImage = submitArgs.Image
	}

	if submitArgs.WorkerCount > 0 {
		autoSelectWorkerPort, err := util.SelectAvailablePortWithDefault(clientset, submitArgs.WorkerPort)
		if err != nil {
			return fmt.Errorf("failed to select worker port: %++v", err)
		}
		submitArgs.WorkerPort = autoSelectWorkerPort
	}

	if submitArgs.UseChief {
		autoSelectChiefPort, err := util.SelectAvailablePortWithDefault(clientset, submitArgs.ChiefPort)
		if err != nil {
			return fmt.Errorf("failed to select chief port: %++v", err)
		}
		submitArgs.ChiefPort = autoSelectChiefPort
		submitArgs.ChiefCount = 1
	}

	if submitArgs.PSCount > 0 {
		autoSelectPsPort, err := util.SelectAvailablePortWithDefault(clientset, submitArgs.PSPort)
		if err != nil {
			return fmt.Errorf("failed to select ps port: %++v", err)
		}
		submitArgs.PSPort = autoSelectPsPort
		if submitArgs.PSImage == "" {
			submitArgs.PSImage = submitArgs.Image
		}
	}

	if submitArgs.UseEvaluator {
		submitArgs.EvaluatorCount = 1
	}

	// check Gang scheduler
	submitArgs.checkGangCapablitiesInCluster()

	return nil
}

// add node selectors
func (submitArgs *submitTFJobArgs) addTFNodeSelectors() {
	submitArgs.TFNodeSelectors = make(map[string]map[string]string)
	for _, role := range []string{"PS", "Worker", "Evaluator", "Chief"} {
		switch role {
		case "PS":
			log.Debugf("psSelectors: %v", psSelectors)
			submitArgs.transformSelectorArrayToMap(psSelectors, "PS")
			break
		case "Worker":
			log.Debugf("workerSelectors: %v", workerSelectors)
			submitArgs.transformSelectorArrayToMap(workerSelectors, "Worker")
			break
		case "Chief":
			log.Debugf("chiefSelectors: %v", chiefSelectors)
			submitArgs.transformSelectorArrayToMap(chiefSelectors, "Chief")
			break
		case "Evaluator":
			log.Debugf("evaluatorSelectors: %v", evaluatorSelectors)
			submitArgs.transformSelectorArrayToMap(evaluatorSelectors, "Evaluator")
			break
		}

	}
}

func (submitArgs *submitTFJobArgs) transformSelectorArrayToMap(selectorArray []string, role string) {
	if len(selectorArray) != 0 {
		submitArgs.TFNodeSelectors[role] = transformSliceToMap(selectorArray, "=")
		return
	}
	if len(submitArgs.NodeSelectors) == 0 {
		submitArgs.TFNodeSelectors[role] = map[string]string{}
		return
	}
	submitArgs.TFNodeSelectors[role] = submitArgs.NodeSelectors

}

func (submitArgs *submitTFJobArgs) addConfigFiles() error {
	return submitArgs.addJobConfigFiles()
}

func (submitArgs *submitTFJobArgs) checkGangCapablitiesInCluster() {
	gangCapablity := false
	if clientset != nil {
		_, err := clientset.AppsV1beta1().Deployments(metav1.NamespaceSystem).Get(gangSchdName, metav1.GetOptions{})
		if err != nil {
			log.Debugf("Failed to find %s due to %v", gangSchdName, err)
		} else {
			log.Debugf("Found %s successfully, the gang scheduler is enabled in the cluster.", gangSchdName)
			gangCapablity = true
		}
	}

	submitArgs.HasGangScheduler = gangCapablity
}

func submitTFJob(args []string, submitArgs *submitTFJobArgs) (err error) {
	// Get runtime name
	runtimeName := getRuntimeName()
	if runtimeName != "" {
		submitArgs.tfRuntime = getTFRuntime(runtimeName)
	}

	err = submitArgs.prepare(args)
	if err != nil {
		return err
	}

	trainer := NewTensorFlowJobTrainer(clientset)
	job, err := trainer.GetTrainingJob(name, namespace)
	if err != nil {
		log.Debugf("Check %s exist due to error %v", name, err)
	}

	if job != nil {
		return fmt.Errorf("the job %s is already exist, please delete it first. use 'arena delete %s'", name, name)
	}

	// the master is also considered as a worker
	// submitArgs.WorkerCount = submitArgs.WorkerCount - 1

	if submitArgs.tfRuntime != nil {
		tfjob_chart = util.GetChartsFolder() + "/" + submitArgs.tfRuntime.getChartName()
	}
	err = workflow.SubmitJob(name, submitArgs.Mode, namespace, submitArgs, tfjob_chart, submitArgs.addHelmOptions()...)
	if err != nil {
		return err
	}

	log.Infof("The Job %s has been submitted successfully", name)
	log.Infof("You can run `arena get %s --type %s` to check the job status", name, submitArgs.Mode)
	return nil
}

func submitTFJobWithHelm(args []string, submitArgs *submitTFJobArgs) (err error) {
	err = submitArgs.prepare(args)
	if err != nil {
		return err
	}

	exist, err := helm.CheckRelease(name)
	if err != nil {
		return err
	}
	if exist {
		return fmt.Errorf("the job %s is already exist, please delete it first. use 'arena delete %s'", name, name)
	}

	// the master is also considered as a worker
	// submitArgs.WorkerCount = submitArgs.WorkerCount - 1

	return helm.InstallRelease(name, namespace, submitArgs, tfjob_chart)
}
