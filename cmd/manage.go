package cmd

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/utils"

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
		if stopSpinnerFn != nil {
			stopSpinnerFn()
		}

		fmt.Printf("\n%s\n\033[2mEnter to confirm, or type feedback to revise:\033[0m\n", result.Proposal)
		fmt.Fprintf(os.Stderr, utils.PromptCaret)
		reader := bufio.NewReader(os.Stdin)
		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if restartSpinnerFn != nil {
			restartSpinnerFn()
		}

		if input == "" {
			return ai.ConfirmResponse{Approved: true}
		}
		return ai.ConfirmResponse{Feedback: input}
	}

	session, err := ai.NewManageSession(dir, confirmFn)
	if err != nil {
		return err
	}
	defer session.SaveMemories()

	reader := bufio.NewReader(os.Stdin)

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
		fmt.Fprintf(os.Stderr, utils.UserEcho, question)
		return send(question)
	}

	// REPL mode
	fmt.Fprintf(os.Stderr, "\033[2mManage mode — read + write access to your vault. exit to quit.\033[0m\n\n")

	followUp := false
	for {
		if followUp {
			fmt.Printf("\n" + utils.PromptHint + "\n")
		}

		input, done := utils.PromptBare(reader)
		if done {
			return nil
		}
		if input == "" {
			continue
		}

		if followUp {
			fmt.Fprintf(os.Stderr, "\033[A\033[2K\033[A\033[2K\033[A\033[2K\r")
		} else {
			fmt.Fprintf(os.Stderr, "\033[A\033[2K\r")
		}
		fmt.Fprintf(os.Stderr, utils.UserEcho, input)

		if err := send(input); err != nil {
			return err
		}

		followUp = true
	}
}
