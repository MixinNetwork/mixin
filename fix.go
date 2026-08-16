package main

import (
	"fmt"
	"strings"
)

type Inquiry struct {
	Title   string
	Content []string
}

func main() {
	q1 := "The formal vulnerability disclosure process and expected report format"
	q2 := "Whether you currently operate an active bug bounty program"
	q3 := "The repositories, smart contracts, infrastructure components, or services that are considered in-scope"
	q4 := "Any testing restrictions, safe harbor terms, or prohibited activities researchers should be aware of"

	content := Inquiry{
		Title:   "Responsible Disclosure & Research Scope Inquiry",
		Content: []string{
			"Hello Mixin Team,",
			"I am reaching out via your official security contact to inquire about your responsible disclosure process and current research scope.",
			"As an independent security researcher focusing on cross-chain and distributed protocol architectures, I am interested in conducting research within clearly defined and authorized boundaries.",
			"Could you please clarify:",
			q1,
			q2,
			q3,
			q4,
			"My intention is to ensure that any security research conducted is fully aligned with your policies and responsible disclosure standards.",
		},
	}

	fmt.Println(strings.Repeat("=", 50))
	fmt.Println("Inquiry Title: " + content.Title)
	fmt.Println(strings.Repeat("-", 50))
	for _, line := range content.Content {
		fmt.Println(line)
	}
	fmt.Println(strings.Repeat("=", 50))
}