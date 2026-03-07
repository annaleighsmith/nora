package cmd

import (
	"fmt"
	"os"
	"strings"

	"n-notes/ai"
	"n-notes/utils"

	"github.com/spf13/cobra"
)

var tagsCmd = &cobra.Command{
	Use:   "tags",
	Short: "List or manage tags",
	RunE:  runTagsList,
}

var tagsAddCmd = &cobra.Command{
	Use:   "add <tag>",
	Short: "Find notes that should have a tag and add it",
	Args:  cobra.ExactArgs(1),
	RunE:  runTagsAdd,
}

func init() {
	rootCmd.AddCommand(tagsCmd)
	tagsCmd.AddCommand(tagsAddCmd)
	tagsAddCmd.Flags().Bool("ai", false, "Use AI to judge relevance (costs tokens)")
}

func runTagsList(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	result, err := utils.ListTags(dir)
	if err != nil {
		return err
	}
	fmt.Print(result)
	return nil
}

func runTagsAdd(cmd *cobra.Command, args []string) error {
	dir, err := loadNotesDir()
	if err != nil {
		return err
	}

	tag := strings.ToLower(strings.TrimSpace(args[0]))
	useAI, _ := cmd.Flags().GetBool("ai")

	// Find candidate notes: match content but don't already have the tag
	candidates, err := utils.FindUntagged(dir, tag)
	if err != nil {
		return err
	}

	if len(candidates) == 0 {
		fmt.Printf("No untagged notes found matching %q.\n", tag)
		return nil
	}

	if useAI {
		candidates, err = aiFilterCandidates(dir, tag, candidates)
		if err != nil {
			return err
		}
		if len(candidates) == 0 {
			fmt.Printf("AI found no relevant untagged notes for %q.\n", tag)
			return nil
		}
	}

	// Preview
	fmt.Printf("Found %d note(s) to tag with %q:\n", len(candidates), tag)
	for _, name := range candidates {
		fmt.Printf("  + %s\n", name)
	}

	fmt.Printf("\nAdd tag %q to these notes? [y/N] ", tag)
	var confirm string
	fmt.Scanln(&confirm)
	if strings.ToLower(confirm) != "y" {
		fmt.Println("Cancelled.")
		return nil
	}

	added := 0
	for _, name := range candidates {
		if err := utils.AddTag(dir, name, tag); err != nil {
			fmt.Fprintf(os.Stderr, "  error tagging %s: %v\n", name, err)
			continue
		}
		added++
	}
	fmt.Printf("Tagged %d note(s) with %q.\n", added, tag)
	return nil
}

func aiFilterCandidates(dir, tag string, candidates []string) ([]string, error) {
	// Build a prompt with the candidate filenames + first lines
	var sb strings.Builder
	for _, name := range candidates {
		preview := utils.FirstLines(dir, name, 5)
		sb.WriteString(fmt.Sprintf("## %s\n%s\n\n", name, preview))
	}

	prompt := fmt.Sprintf(`Given the tag %q, which of these notes are genuinely related to this topic? Only include notes where the tag is clearly relevant, not just a passing mention.

Return ONLY the filenames, one per line. No explanations, no bullets, no numbering. If none are relevant, return "none".

%s`, tag, sb.String())

	result, err := ai.QuickQuery(prompt)
	if err != nil {
		return nil, fmt.Errorf("AI error: %w", err)
	}

	if strings.TrimSpace(strings.ToLower(result)) == "none" {
		return nil, nil
	}

	// Parse filenames from response
	validCandidates := make(map[string]bool)
	for _, c := range candidates {
		validCandidates[c] = true
	}

	var filtered []string
	for _, line := range strings.Split(result, "\n") {
		name := strings.TrimSpace(line)
		if validCandidates[name] {
			filtered = append(filtered, name)
		}
	}
	return filtered, nil
}
