// +build unit

package install

import (
	"errors"
	"net/url"
	"reflect"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/newrelic/newrelic-cli/internal/install/discovery"
	"github.com/newrelic/newrelic-cli/internal/install/execution"
	"github.com/newrelic/newrelic-cli/internal/install/recipes"
	"github.com/newrelic/newrelic-cli/internal/install/types"
	"github.com/newrelic/newrelic-cli/internal/install/ux"
	"github.com/newrelic/newrelic-cli/internal/install/validation"
)

var (
	testRecipeName = "Test Recipe"
	testRecipeFile = &recipes.RecipeFile{
		Name: testRecipeName,
	}

	d               = discovery.NewMockDiscoverer()
	l               = discovery.NewMockFileFilterer()
	f               = recipes.NewMockRecipeFetcher()
	e               = execution.NewMockRecipeExecutor()
	v               = validation.NewMockRecipeValidator()
	ff              = recipes.NewMockRecipeFileFetcher()
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status          = execution.NewStatusRollup(statusReporters)
	p               = ux.NewMockPrompter()
	s               = ux.NewMockProgressIndicator()
)

func TestInstall(t *testing.T) {
	assert.True(t, true)
}

func TestNewRecipeInstaller_InstallerContextFields(t *testing.T) {
	ic := InstallerContext{
		RecipePaths:        []string{"testRecipePath"},
		RecipeNames:        []string{"testRecipeName"},
		SkipDiscovery:      true,
		SkipIntegrations:   true,
		SkipLoggingInstall: true,
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}

	require.True(t, reflect.DeepEqual(ic, i.InstallerContext))
}

func TestShouldGetRecipeFromURL(t *testing.T) {
	ic := InstallerContext{}
	ff = recipes.NewMockRecipeFileFetcher()
	ff.FetchRecipeFileFunc = fetchRecipeFileFunc
	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}

	recipe, err := i.recipeFromPath("http://recipe/URL")
	require.NoError(t, err)
	require.NotNil(t, recipe)
	require.Equal(t, recipe.Name, testRecipeName)
}

func TestShouldGetRecipeFromFile(t *testing.T) {
	ic := InstallerContext{}
	ff = recipes.NewMockRecipeFileFetcher()
	ff.LoadRecipeFileFunc = loadRecipeFileFunc
	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}

	recipe, err := i.recipeFromPath("file.txt")
	require.NoError(t, err)
	require.NotNil(t, recipe)
	require.Equal(t, recipe.Name, testRecipeName)
}

func TestInstall_Basic(t *testing.T) {
	ic := InstallerContext{}
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecipeVals = []types.Recipe{
		{Name: infraAgentRecipeName},
		{Name: loggingRecipeName},
	}
	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, f.FetchRecipeNameCount[infraAgentRecipeName], 1)
	require.Equal(t, f.FetchRecipeNameCount[loggingRecipeName], 1)
}

func TestInstall_ReportRecipesAvailable(t *testing.T) {
	ic := InstallerContext{}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipesAvailableCallCount)
}

func TestInstall_ReportRecipeInstalled(t *testing.T) {
	ic := InstallerContext{}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           testRecipeName,
		DisplayName:    testRecipeName,
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{
		{
			Name:           infraAgentRecipeName,
			DisplayName:    infraAgentRecipeName,
			ValidationNRQL: "testNrql",
		},
		{
			Name:           loggingRecipeName,
			DisplayName:    loggingRecipeName,
			ValidationNRQL: "testNrql",
		},
	}

	p = &ux.MockPrompter{
		PromptYesNoVal:       true,
		PromptMultiSelectAll: true,
	}

	v = validation.NewMockRecipeValidator()

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 3, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func TestInstall_ReportRecipeFailed(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           testRecipeName,
		DisplayName:    testRecipeName,
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{
		{
			Name:           infraAgentRecipeName,
			DisplayName:    infraAgentRecipeName,
			ValidationNRQL: "testNrql",
		},
		{
			Name:           loggingRecipeName,
			DisplayName:    loggingRecipeName,
			ValidationNRQL: "testNrql",
		},
	}

	p = &ux.MockPrompter{
		PromptYesNoVal:       true,
		PromptMultiSelectAll: true,
	}

	v = validation.NewMockRecipeValidator()
	v.ValidateErr = errors.New("validationErr")

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.Error(t, err)
	require.Equal(t, 1, v.ValidateCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeFailedCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
}

func TestInstall_ReportComplete(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{}
	f.FetchRecipeVals = []types.Recipe{{}, {}}

	v = validation.NewMockRecipeValidator()

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportCompleteCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
}

func TestInstall_ReportCompleteError(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{}
	f.FetchRecipeVals = []types.Recipe{
		{
			Name:           infraAgentRecipeName,
			ValidationNRQL: "testNrql",
		},
	}

	p = &ux.MockPrompter{
		PromptYesNoVal: true,
	}

	v = validation.NewMockRecipeValidator()
	v.ValidateErr = errors.New("test error")

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.Error(t, err)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportCompleteCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeFailedCallCount)
}

func TestInstall_ReportRecipeSkipped(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           testRecipeName,
		DisplayName:    "test displayName",
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{
		{
			Name:        infraAgentRecipeName,
			DisplayName: "Infra Recipe",
		},
		{
			Name:        loggingRecipeName,
			DisplayName: "Logging Recipe",
		},
	}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal:       true,
		PromptMultiSelectAll: true,
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstallingCallCount)
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func TestInstall_ReportRecipeSkipped_skipping_everything(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
		SkipIntegrations:   true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           "test-recipe",
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{{Name: "one"}, {Name: "two"}}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal:       false,
		PromptMultiSelectVal: []string{},
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 3, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
	// The infra agent always gets installed
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func TestInstall_ReportRecipeSkipped_multiselect(t *testing.T) {
	ic := InstallerContext{
		SkipLoggingInstall: true,
	}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           testRecipeName,
		DisplayName:    testRecipeName,
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{{Name: "one"}, {Name: "two"}}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal:       true,
		PromptMultiSelectVal: []string{testRecipeName},
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
	// The infra agent always gets installed
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstallingCallCount)
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func TestInstall_ReportRecipeRecommended(t *testing.T) {
	ic := InstallerContext{}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{
		{
			Name:           testRecipeName,
			DisplayName:    testRecipeName,
			ValidationNRQL: "testNrql",
			InstallTargets: []types.OpenInstallationRecipeInstallTarget{
				{
					Type: types.OpenInstallationTargetTypeTypes.HOST,
				},
			},
		},
		{
			Name:           "java-java-java",
			ValidationNRQL: "testNrql",
			InstallTargets: []types.OpenInstallationRecipeInstallTarget{
				{
					Type: types.OpenInstallationTargetTypeTypes.APPLICATION,
				},
			},
		},
	}
	f.FetchRecipeVals = []types.Recipe{{Name: "one"}, {Name: "two"}}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal:       true,
		PromptMultiSelectVal: []string{testRecipeName},
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	// The infra agent is always installed
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportInstalled[testRecipeName])
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportInstalled["one"])
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeRecommendedCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecommended["java-java-java"])
}

func TestInstallAdvancedMode_bounce_on_enter(t *testing.T) {
	ic := InstallerContext{}
	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           "test-recipe",
		ValidationNRQL: "testNrql",
	}}

	f.FetchRecipeVals = []types.Recipe{
		{
			Name:           infraAgentRecipeName,
			ValidationNRQL: "testNrql",
		},
		{
			Name:           loggingRecipeName,
			ValidationNRQL: "testNrql",
		},
	}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal: true,
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 2, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
	require.Equal(t, 1, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func TestInstall_ReportRecipeSkipped_assumeYes(t *testing.T) {
	ic := InstallerContext{
		AssumeYes: true,
	}

	statusReporters = []execution.StatusReporter{execution.NewMockStatusReporter()}
	status = execution.NewStatusRollup(statusReporters)
	f = recipes.NewMockRecipeFetcher()
	f.FetchRecommendationsVal = []types.Recipe{{
		Name:           testRecipeName,
		DisplayName:    "test displayName",
		ValidationNRQL: "testNrql",
	}}
	f.FetchRecipeVals = []types.Recipe{
		{
			Name:        infraAgentRecipeName,
			DisplayName: "Infra Recipe",
		},
		{
			Name:        loggingRecipeName,
			DisplayName: "Logging Recipe",
		},
	}

	v = validation.NewMockRecipeValidator()
	p = &ux.MockPrompter{
		PromptYesNoVal: true,
		// PromptMultiSelectAll: true,
	}

	i := RecipeInstaller{ic, d, l, f, e, v, ff, status, p, s}
	err := i.Install()
	require.NoError(t, err)
	require.Equal(t, 0, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeSkippedCallCount)
	require.Equal(t, 3, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstallingCallCount)
	require.Equal(t, 3, statusReporters[0].(*execution.MockStatusReporter).ReportRecipeInstalledCallCount)
}

func fetchRecipeFileFunc(recipeURL *url.URL) (*recipes.RecipeFile, error) {
	return testRecipeFile, nil
}

func loadRecipeFileFunc(filename string) (*recipes.RecipeFile, error) {
	return testRecipeFile, nil
}
