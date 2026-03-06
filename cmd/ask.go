package cmd

import (
	"bufio"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"n-notes/ai"
	"n-notes/notes"

	"github.com/spf13/cobra"
)

var askCmd = &cobra.Command{
	Use:   "ask [question...]",
	Short: "Ask a question about your notes (AI-powered)",
	Args:  cobra.MinimumNArgs(1),
	RunE:  runAsk,
}

func init() {
	rootCmd.AddCommand(askCmd)
	rootCmd.Flags().BoolP("ask", "q", false, "Ask a question about your notes")
}

func runAsk(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	session, err := ai.NewAskSession(dir)
	if err != nil {
		return err
	}

	question := strings.Join(args, " ")
	reader := bufio.NewReader(os.Stdin)
	defer session.SaveMemories()

	for {
		if question == "" {
			return nil
		}

		files, err := session.Ask(question)
		if err != nil {
			return err
		}

		// Browse / follow-up prompt
		fmt.Println()
		if len(files) > 0 {
			fmt.Printf("\033[2m%d note(s) cited. [e]dit / [l]ook / follow-up / Enter to quit:\033[0m ", len(files))
		} else {
			fmt.Printf("\n\033[2mFollow-up / Enter to quit:\033[0m ")
		}

		input, _ := reader.ReadString('\n')
		input = strings.TrimSpace(input)

		if input == "" {
			return nil
		}

		// Browse mode
		if len(files) > 0 && (input == "e" || input == "l") {
			browseFiles(files, input)

			// After browsing, offer follow-up
			fmt.Printf("\n\033[2mFollow-up / Enter to quit:\033[0m ")
			input, _ = reader.ReadString('\n')
			input = strings.TrimSpace(input)
			if input == "" {
				return nil
			}
		}

		question = input
	}
}

func browseFiles(files []string, mode string) {
	for {
		selected, err := notes.PickFrom(files, true)
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
			fmt.Println(notes.Render(string(content)))
		}
	}
}
