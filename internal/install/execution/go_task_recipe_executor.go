package execution

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"

	"github.com/go-task/task/v3"
	taskargs "github.com/go-task/task/v3/args"
	"github.com/go-task/task/v3/taskfile"
	"github.com/manifoldco/promptui"
	log "github.com/sirupsen/logrus"
	"gopkg.in/yaml.v2"

	"github.com/newrelic/newrelic-cli/internal/configuration"
	"github.com/newrelic/newrelic-cli/internal/install/recipes"
	"github.com/newrelic/newrelic-cli/internal/install/types"
)

// GoTaskRecipeExecutor is an implementation of the recipeExecutor interface that
// uses the go-task module to execute the steps defined in each recipe.
type GoTaskRecipeExecutor struct{}

// NewGoTaskRecipeExecutor returns a new instance of GoTaskRecipeExecutor.
func NewGoTaskRecipeExecutor() *GoTaskRecipeExecutor {
	return &GoTaskRecipeExecutor{}
}

func (re *GoTaskRecipeExecutor) Prepare(ctx context.Context, m types.DiscoveryManifest, r types.Recipe, assumeYes bool) (types.RecipeVars, error) {
	vars := types.RecipeVars{}

	results := []types.RecipeVars{}

	systemInfoResult := varsFromSystemInfo(m)

	profileResult, err := varsFromProfile()
	if err != nil {
		return types.RecipeVars{}, err
	}

	recipeResult, err := varsFromRecipe(r)
	if err != nil {
		return types.RecipeVars{}, err
	}

	f, err := recipes.RecipeToRecipeFile(r)
	if err != nil {
		return types.RecipeVars{}, err
	}

	inputVarsResult, err := varsFromInput(f.InputVars, assumeYes)
	if err != nil {
		return types.RecipeVars{}, err
	}

	results = append(results, systemInfoResult)
	results = append(results, profileResult)
	results = append(results, recipeResult)
	results = append(results, inputVarsResult)

	for _, result := range results {
		for k, v := range result {
			vars[k] = v
		}
	}

	return vars, nil
}

func (re *GoTaskRecipeExecutor) Execute(ctx context.Context, m types.DiscoveryManifest, r types.Recipe, recipeVars types.RecipeVars) error {
	log.Debugf("executing recipe %s", r.Name)

	f, err := recipes.RecipeToRecipeFile(r)
	if err != nil {
		return err
	}

	out, err := yaml.Marshal(f.Install)
	if err != nil {
		return err
	}

	// Create a temporary task file.
	file, err := ioutil.TempFile("", r.Name)
	defer os.Remove(file.Name())
	if err != nil {
		return err
	}

	_, err = file.Write(out)
	if err != nil {
		return err
	}

	var stderrCapture bytes.Buffer
	var stdoutCapture bytes.Buffer

	e := task.Executor{
		Entrypoint: file.Name(),
		Stderr:     &stderrCapture,
		Stdout:     &stdoutCapture,
	}

	// Only pipe child process output streams for the chattier log levels
	if log.GetLevel() > log.InfoLevel {
		e.Stdout = os.Stdout
		e.Stderr = os.Stderr
	}

	if err = e.Setup(); err != nil {
		return err
	}

	var tf taskfile.Taskfile
	err = yaml.Unmarshal(out, &tf)
	if err != nil {
		return err
	}

	calls, globals := taskargs.ParseV3()
	e.Taskfile.Vars.Merge(globals)

	for k, val := range recipeVars {
		e.Taskfile.Vars.Set(k, taskfile.Var{Static: val})
	}

	if err := e.Run(ctx, calls...); err != nil {
		if log.GetLevel() > log.InfoLevel {
			stderr := stderrCapture.String()
			if stderr != "" {
				log.Error(stderr)
			}
		}
		return err
	}

	return nil
}

func varsFromProfile() (types.RecipeVars, error) {
	v, err := configuration.GetActiveProfileValue(configuration.LicenseKey)
	if err != nil {
		return nil, err
	}

	licenseKey := v.(string)
	if licenseKey == "" {
		return types.RecipeVars{}, errors.New("license key not found in default profile")
	}

	accountID, err := configuration.GetActiveProfileValue(configuration.AccountID)
	if err != nil {
		return nil, err
	}

	apiKey, err := configuration.GetActiveProfileValue(configuration.APIKey)
	if err != nil {
		return nil, err
	}

	region, err := configuration.GetActiveProfileValue(configuration.Region)
	if err != nil {
		return nil, err
	}

	vars := make(types.RecipeVars)

	vars["NEW_RELIC_LICENSE_KEY"] = licenseKey
	vars["NEW_RELIC_ACCOUNT_ID"] = strconv.Itoa(int(accountID.(float64)))
	vars["NEW_RELIC_API_KEY"] = apiKey.(string)
	vars["NEW_RELIC_REGION"] = region.(string)

	return vars, nil
}

func varsFromSystemInfo(m types.DiscoveryManifest) types.RecipeVars {
	vars := make(types.RecipeVars)

	vars["HOSTNAME"] = m.Hostname
	vars["OS"] = m.OS
	vars["PLATFORM"] = m.Platform
	vars["PLATFORM_FAMILY"] = m.PlatformFamily
	vars["PLATFORM_VERSION"] = m.PlatformVersion
	vars["KERNEL_ARCH"] = m.KernelArch
	vars["KERNEL_VERSION"] = m.KernelVersion

	return vars
}

func varsFromRecipe(r types.Recipe) (types.RecipeVars, error) {
	vars := make(types.RecipeVars)

	for k, x := range r.Vars {
		varData, err := yaml.Marshal(x)
		if err != nil {
			return types.RecipeVars{}, err
		}

		vars[k] = string(varData)
	}

	return vars, nil
}

func varsFromInput(inputVars []recipes.VariableConfig, assumeYes bool) (types.RecipeVars, error) {
	vars := make(types.RecipeVars)

	for _, envConfig := range inputVars {
		var err error
		envValue := os.Getenv(envConfig.Name)

		if envValue != "" {
			vars[envConfig.Name] = envValue
			continue
		}

		if assumeYes {
			if envConfig.Default == "" {
				return types.RecipeVars{}, fmt.Errorf("no default value for environment variable %s and none provided", envConfig.Name)
			}

			log.WithFields(log.Fields{
				"name":    envConfig.Name,
				"default": envConfig.Default,
			}).Debug("required env var not found, using default")

			envValue = envConfig.Default
		} else {
			log.WithFields(log.Fields{
				"name": envConfig.Name,
			}).Debug("required env var not found, prompting for input")

			envValue, err = varFromPrompt(envConfig)
			if err != nil {
				return types.RecipeVars{}, fmt.Errorf("prompt failed: %s", err)
			}
		}

		vars[envConfig.Name] = envValue
	}

	return vars, nil
}

func varFromPrompt(envConfig recipes.VariableConfig) (string, error) {
	msg := fmt.Sprintf("value for %s required", envConfig.Name)

	if envConfig.Prompt != "" {
		msg = fmt.Sprintf("%s: %s", envConfig.Name, envConfig.Prompt)
	}

	prompt := promptui.Prompt{
		Label: msg,
	}

	if envConfig.Secret {
		prompt.HideEntered = true
	}

	if envConfig.Default != "" {
		prompt.Default = envConfig.Default
	}

	return prompt.Run()
}
