package common

const (
	AgentContainerName        = "promotion-agent"
	AgentContainerPort        = 9764
	AgentPodNameSuffix        = "-promotion-agent"
	AgentReadHeaderTimeoutSec = 10
	APIPathPrefix             = "/api/v1/"
	AuthDir                   = "/var/run/kargo"
	AuthFilename              = "token"
	AuthVolumeName            = "kargo-step-plugin-auth"
	ConfigMapLabelKey         = "kargo-extended.code.org/configmap-type"
	ConfigMapLabelValue       = "StepPlugin"
	ConfigMapNameSuffix       = "-step-plugin"
	ConfigMapFilenameSuffix   = "-step-plugin-configmap.yaml"
	DefaultAgentRetrySeconds  = 2
	EnvVarPluginTargets       = "KARGO_STEP_PLUGIN_TARGETS"
	MethodStepExecute         = "step.execute"
	ServiceAccountMountPath   = "/var/run/secrets/kubernetes.io/serviceaccount"
	ServiceAccountVolumeName  = "kargo-service-account"
	WorkDir                   = "/workspace"
	WorkDirVolumeName         = "kargo-workdir"
)
