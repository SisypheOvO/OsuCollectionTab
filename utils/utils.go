package utils

import (
	"fmt"
	"os"
	"strings"
)

func PathExists(path string) bool {
	_, err := os.Stat(path)
	return !os.IsNotExist(err)
}

// Calculate the intersection of two sets
func SetDifference(setA, setB map[string]bool) map[string]bool {
	result := make(map[string]bool)
	for k := range setA {
		if !setB[k] {
			result[k] = true
		}
	}
	return result
}

func PromptDownloadType() string {
	fmt.Println("Please select the download type:")
	fmt.Println("1. with video (full)")
	fmt.Println("2. without video (novideo)")
	fmt.Println("3. mini")

	var choice string
	fmt.Print("Enter your choice (1/2/3): ")
	fmt.Scanln(&choice)

	switch strings.TrimSpace(choice) {
	case "1":
		return "full"
	case "2":
		return "novideo"
	case "3":
		return "mini"
	default:
		fmt.Println("Invalid choice, defaulting to 'full'")
		return "full"
	}
}