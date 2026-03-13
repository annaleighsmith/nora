package cmd

import (
	"fmt"
	"os"
	"strings"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/utils"

	"github.com/charmbracelet/huh"
	"github.com/spf13/cobra"
)

var manageCmd = &cobra.Command{
	Use:   "manage [message...]",
	Short: "Manage your vault with AI (read + write)",
	Long: `Interactive AI session with full read and write access to your notes.

  nora manage                              Interactive REPL
  nora manage fix all broken frontmatter   One-shot mode
  nora manage delete old scratch notes     One-shot mode`,
	Args: cobra.ArbitraryArgs,
	RunE: runManage,
}

func init() {
	rootCmd.AddCommand(manageCmd)
}

func runManage(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	// Spinner control shared between the REPL and the confirm function.
	// When a write tool needs confirmation, the spinner pauses so the
	// prompt is visible, then resumes after the user responds.
	var stopSpinnerFn func()
	var restartSpinnerFn func()

	confirmFn := func(result ai.ToolResult) ai.ConfirmResponse {
		// Non-interactive: auto-approve (the agent intended the action)
		if !utils.IsInteractive() {
			return ai.ConfirmResponse{Approved: true}
		}

		if stopSpinnerFn != nil {
			stopSpinnerFn()
		}

		if result.ProposalFile != "" && utils.IsTTY() {
			meta := utils.ParseNoteMeta(result.Proposal, result.ProposalFile)
			body := utils.StripFrontmatter(result.Proposal)
			rendered := utils.RenderWidth(body, utils.TermWidth()-6)
			fmt.Printf("\n%s", utils.FrameContent(rendered, meta))
		} else {
			fmt.Printf("\n%s\n", utils.Render(result.Proposal))
		}

		var input string
		err := huh.NewInput().
			Title("Enter to confirm, or type feedback to revise").
			Prompt("> ").
			Value(&input).
			Run()
		if err != nil {
			return ai.ConfirmResponse{Approved: true}
		}
		input = strings.TrimSpace(input)

		if input == "" {
			return ai.ConfirmResponse{Approved: true}
		}

		if restartSpinnerFn != nil {
			restartSpinnerFn()
		}
		return ai.ConfirmResponse{Feedback: input}
	}

	session, err := ai.NewManageSession(dir, confirmFn)
	if err != nil {
		return err
	}
	defer session.SaveMemories()

	send := func(question string) error {
		stop := utils.StartSpinner("Thinking...")
		stopSpinnerFn = stop
		restartSpinnerFn = func() {
			stop = utils.StartSpinner("Thinking...")
			stopSpinnerFn = stop
		}

		_, err := session.Send(question)

		stopSpinnerFn = nil
		restartSpinnerFn = nil
		stop()
		return err
	}

	// One-shot mode: args become the first (and only) message
	if len(args) > 0 {
		question := strings.Join(args, " ")
		utils.UserEcholn(os.Stderr, question)
		return send(question)
	}

	// REPL mode requires interactive terminal
	if !utils.IsInteractive() {
		return fmt.Errorf("nora manage requires a message argument when not running in a terminal")
	}
	utils.Dimf("Manage mode — read + write access to your vault. exit to quit.\n\n")

	followUp := false
	for {
		if followUp {
			fmt.Printf("\n%s\n", utils.PromptHint)
		}

		input, done := utils.PromptBare()
		if done {
			return nil
		}
		if input == "" {
			continue
		}

		if followUp {
			utils.ClearLinesUp(os.Stderr, 3)
		} else {
			utils.ClearLinesUp(os.Stderr, 1)
		}
		utils.UserEcholn(os.Stderr, input)

		if err := send(input); err != nil {
			return err
		}

		followUp = true
	}
}
