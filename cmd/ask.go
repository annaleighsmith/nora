package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/annaleighsmith/nora/ai"
	"github.com/annaleighsmith/nora/utils"

	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask [question...]",
	Short: "Ask a question about your notes (AI-powered)",
	Long: `Ask a question about your notes. AI searches and reads your vault to answer.

  nora ask what are my project ideas
  nora ask -f my-note.md summarize this
  nora ask -f my-note.md
  nora ask -f`,
	Args: cobra.ArbitraryArgs,
	RunE: runAsk,
}

func init() {
	askCmd.Flags().StringP("file", "f", "", "Scope to a specific note (filename or fzf pick if empty)")
	askCmd.Flag("file").NoOptDefVal = " " // -f with no value triggers fzf pick
	rootCmd.AddCommand(askCmd)
}

func runAsk(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	session, err := ai.NewSession(dir)
	if err != nil {
		return err
	}
	defer session.SaveMemories()

	// File-scoped mode: preload a note into context
	fileFlag, _ := cmd.Flags().GetString("file")
	if cmd.Flags().Changed("file") {
		filePath, question, err := resolveFileAndQuestion(dir, fileFlag, args)
		if err != nil {
			return err
		}
		if filePath == "" {
			return nil
		}

		content, err := os.ReadFile(filePath)
		if err != nil {
			return fmt.Errorf("could not read %s: %w", filePath, err)
		}

		filename := filepath.Base(filePath)
		session.PreloadFile(filename, string(content))
		fmt.Fprintf(os.Stderr, "\033[2mLoaded %s\033[0m\n\n", filename)

		// If no question from args, prompt for one
		if question == "" {
			reader := bufio.NewReader(os.Stdin)
			fmt.Fprintf(os.Stderr, "\033[2mAsk about %s:\033[0m\n", filename)
			input, done := utils.PromptBare(reader)
			if done || input == "" {
				return nil
			}
			question = input
			fmt.Fprintf(os.Stderr, "\033[A\033[2K\033[A\033[2K\r")
		}

		return chatLoop(session, question)
	}

	// Normal mode: question from args
	question := strings.Join(args, " ")
	if question == "" {
		return cmd.Help()
	}

	return chatLoop(session, question)
}

func resolveFileAndQuestion(dir, fileFlag string, args []string) (string, string, error) {
	var filePath string
	var question string

	if strings.TrimSpace(fileFlag) != "" {
		full := filepath.Join(dir, fileFlag)
		if _, err := os.Stat(full); err != nil {
			return "", "", fmt.Errorf("file not found: %s", fileFlag)
		}
		filePath = full
		question = strings.Join(args, " ")
	} else {
		picked, err := utils.Pick(dir, "", true)
		if err != nil || picked == "" {
			return "", "", err
		}
		filePath = picked
		question = strings.Join(args, " ")
	}

	return filePath, question, nil
}

func chatLoop(session *ai.Session, question string) error {
	reader := bufio.NewReader(os.Stdin)
	followUp := false

	for {
		if question == "" {
			continue
		}

		if followUp {
			fmt.Fprintf(os.Stderr, "\033[A\033[2K\033[A\033[2K\r")
		}
		fmt.Fprintf(os.Stderr, utils.UserEcho, question)

		stop := utils.StartSpinner("Thinking...")
		files, err := session.Send(question)
		stop()
		if err != nil {
			return err
		}

		fmt.Println()

		input, done := utils.PromptFollowUp(reader, len(files))
		if done {
			return nil
		}

		if len(files) > 0 && (input == "e" || input == "l") {
			browseFiles(files, input)

			input, done = utils.PromptFollowUp(reader, 0)
			if done {
				return nil
			}
		}

		if input == "" {
			continue
		}

		followUp = true
		question = input
	}
}

func browseFiles(files []string, mode string) {
	for {
		selected, err := utils.PickFrom(files, true)
		if err != nil || selected == "" {
			return
		}

		if mode == "e" {
			editor := os.Getenv("EDITOR")
			if editor == "" {
				editor = "nvim"
			}
			editorCmd := exec.Command(editor, selected)
			editorCmd.Stdin = os.Stdin
			editorCmd.Stdout = os.Stdout
			editorCmd.Stderr = os.Stderr
			editorCmd.Run()
		} else {
			content, err := os.ReadFile(selected)
			if err != nil {
				fmt.Fprintf(os.Stderr, "could not read %s: %v\n", selected, err)
				continue
			}
			fmt.Printf("\033[2m%s\033[0m\n", filepath.Base(selected))
			fmt.Println(utils.Render(string(content)))
		}
	}
}
