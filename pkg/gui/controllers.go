package gui

import (
	"strings"

	"github.com/jesseduffield/gocui"
	"github.com/jesseduffield/lazygit/pkg/commands/models"
	"github.com/jesseduffield/lazygit/pkg/gui/controllers"
	"github.com/jesseduffield/lazygit/pkg/gui/controllers/helpers"
	"github.com/jesseduffield/lazygit/pkg/gui/modes/cherrypicking"
	"github.com/jesseduffield/lazygit/pkg/gui/services/custom_commands"
)

func (gui *Gui) resetControllers() {
	helperCommon := gui.c
	osCommand := gui.os
	model := gui.State.Model
	refsHelper := helpers.NewRefsHelper(
		helperCommon,
		gui.git,
		gui.State.Contexts,
		model,
	)

	rebaseHelper := helpers.NewMergeAndRebaseHelper(helperCommon, gui.State.Contexts, gui.git, gui.takeOverMergeConflictScrolling, refsHelper)
	gui.helpers = &helpers.Helpers{
		Refs:           refsHelper,
		Host:           helpers.NewHostHelper(helperCommon, gui.git),
		PatchBuilding:  helpers.NewPatchBuildingHelper(helperCommon, gui.git),
		Bisect:         helpers.NewBisectHelper(helperCommon, gui.git),
		Suggestions:    helpers.NewSuggestionsHelper(helperCommon, model, gui.refreshSuggestions),
		Files:          helpers.NewFilesHelper(helperCommon, gui.git, osCommand),
		WorkingTree:    helpers.NewWorkingTreeHelper(helperCommon, gui.git, model),
		Tags:           helpers.NewTagsHelper(helperCommon, gui.git),
		GPG:            helpers.NewGpgHelper(helperCommon, gui.os, gui.git),
		MergeAndRebase: rebaseHelper,
		CherryPick: helpers.NewCherryPickHelper(
			helperCommon,
			gui.git,
			gui.State.Contexts,
			func() *cherrypicking.CherryPicking { return gui.State.Modes.CherryPicking },
			rebaseHelper,
		),
	}

	gui.CustomCommandsClient = custom_commands.NewClient(
		helperCommon,
		gui.os,
		gui.git,
		gui.State.Contexts,
		gui.helpers,
		gui.getKey,
	)

	common := controllers.NewControllerCommon(
		helperCommon,
		osCommand,
		gui.git,
		gui.helpers,
		model,
		gui.State.Contexts,
		gui.State.Modes,
	)

	syncController := controllers.NewSyncController(
		common,
		gui.getSuggestedRemote,
	)

	submodulesController := controllers.NewSubmodulesController(
		common,
		gui.enterSubmodule,
	)

	bisectController := controllers.NewBisectController(common)

	getSavedCommitMessage := func() string {
		return gui.State.savedCommitMessage
	}

	getCommitMessage := func() string {
		return strings.TrimSpace(gui.Views.CommitMessage.TextArea.GetContent())
	}

	setCommitMessage := gui.getSetTextareaTextFn(func() *gocui.View { return gui.Views.CommitMessage })

	onCommitAttempt := func(message string) {
		gui.State.savedCommitMessage = message
		gui.Views.CommitMessage.ClearTextArea()
	}

	onCommitSuccess := func() {
		gui.State.savedCommitMessage = ""
	}

	commitMessageController := controllers.NewCommitMessageController(
		common,
		getCommitMessage,
		onCommitAttempt,
		onCommitSuccess,
	)

	remoteBranchesController := controllers.NewRemoteBranchesController(common)

	menuController := controllers.NewMenuController(common)
	localCommitsController := controllers.NewLocalCommitsController(common, syncController.HandlePull)
	tagsController := controllers.NewTagsController(common)
	filesController := controllers.NewFilesController(
		common,
		gui.enterSubmodule,
		setCommitMessage,
		getSavedCommitMessage,
		gui.switchToMerge,
	)
	remotesController := controllers.NewRemotesController(
		common,
		func(branches []*models.RemoteBranch) { gui.State.Model.RemoteBranches = branches },
	)
	undoController := controllers.NewUndoController(common)
	globalController := controllers.NewGlobalController(common)
	branchesController := controllers.NewBranchesController(common)
	gitFlowController := controllers.NewGitFlowController(common)
	filesRemoveController := controllers.NewFilesRemoveController(common)
	stashController := controllers.NewStashController(common)
	commitFilesController := controllers.NewCommitFilesController(common)

	setSubCommits := func(commits []*models.Commit) { gui.State.Model.SubCommits = commits }

	for _, context := range []controllers.CanSwitchToSubCommits{
		gui.State.Contexts.Branches,
		gui.State.Contexts.RemoteBranches,
		gui.State.Contexts.Tags,
		gui.State.Contexts.ReflogCommits,
	} {
		controllers.AttachControllers(context, controllers.NewSwitchToSubCommitsController(
			common, setSubCommits, context,
		))
	}

	for _, context := range []controllers.CanSwitchToDiffFiles{
		gui.State.Contexts.LocalCommits,
		gui.State.Contexts.SubCommits,
		gui.State.Contexts.Stash,
	} {
		controllers.AttachControllers(context, controllers.NewSwitchToDiffFilesController(
			common, gui.SwitchToCommitFilesContext, context,
		))
	}

	for _, context := range []controllers.ContainsCommits{
		gui.State.Contexts.LocalCommits,
		gui.State.Contexts.ReflogCommits,
		gui.State.Contexts.SubCommits,
	} {
		controllers.AttachControllers(context, controllers.NewBasicCommitsController(common, context))
	}

	controllers.AttachControllers(gui.State.Contexts.Files, filesController, filesRemoveController)
	controllers.AttachControllers(gui.State.Contexts.Tags, tagsController)
	controllers.AttachControllers(gui.State.Contexts.Submodules, submodulesController)
	controllers.AttachControllers(gui.State.Contexts.LocalCommits, localCommitsController, bisectController)
	controllers.AttachControllers(gui.State.Contexts.Branches, branchesController, gitFlowController)
	controllers.AttachControllers(gui.State.Contexts.LocalCommits, localCommitsController, bisectController)
	controllers.AttachControllers(gui.State.Contexts.CommitFiles, commitFilesController)
	controllers.AttachControllers(gui.State.Contexts.Remotes, remotesController)
	controllers.AttachControllers(gui.State.Contexts.Stash, stashController)
	controllers.AttachControllers(gui.State.Contexts.Menu, menuController)
	controllers.AttachControllers(gui.State.Contexts.CommitMessage, commitMessageController)
	controllers.AttachControllers(gui.State.Contexts.RemoteBranches, remoteBranchesController)
	controllers.AttachControllers(gui.State.Contexts.Global, syncController, undoController, globalController)

	// this must come last so that we've got our click handlers defined against the context
	listControllerFactory := controllers.NewListControllerFactory(gui.c)
	for _, context := range gui.getListContexts() {
		controllers.AttachControllers(context, listControllerFactory.Create(context))
	}
}
