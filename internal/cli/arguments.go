package cli

import "strings"

// PrepareArguments preserves Facter's command precedence: known command flags are
// moved before normal flags, and bare fact names become query arguments.
func PrepareArguments(args []string) []string {
	prepared := expandShortOptions(args)
	if !containsKnownTaskOrMappedFlag(prepared) {
		prepared = append([]string{"query"}, prepared...)
	}

	priority := make([]string, 0, len(prepared))
	normal := make([]string, 0, len(prepared))
	for i := 0; i < len(prepared); i++ {
		arg := prepared[i]
		if mappedFlags[arg] || tasks[arg] {
			priority = append(priority, arg)
			continue
		}
		normal = append(normal, arg)
		if optionTakesSeparateValue(arg) && i+1 < len(prepared) {
			i++
			normal = append(normal, prepared[i])
		}
	}
	return append(priority, normal...)
}

func expandShortOptions(args []string) []string {
	expanded := make([]string, 0, len(args))
	for _, arg := range args {
		if len(arg) <= 2 || arg[0] != '-' || arg[1] == '-' || strings.ContainsRune(arg, '=') {
			expanded = append(expanded, arg)
			continue
		}
		if shortOptionTakesAttachedValue(arg[1]) {
			expanded = append(expanded, arg[:2], arg[2:])
			continue
		}
		for _, flag := range arg[1:] {
			expanded = append(expanded, "-"+string(flag))
		}
	}
	return expanded
}

func shortOptionTakesAttachedValue(flag byte) bool {
	switch flag {
	case 'c', 'l':
		return true
	default:
		return false
	}
}

var tasks = map[string]bool{
	"help":              true,
	"query":             true,
	"version":           true,
	"man":               true,
	"list_block_groups": true,
	"list_cache_groups": true,
}

var mappedFlags = map[string]bool{
	"-h":                  true,
	"--help":              true,
	"--man":               true,
	"-v":                  true,
	"--version":           true,
	"--list-block-groups": true,
	"--list-cache-groups": true,
}

func containsKnownTaskOrMappedFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if tasks[arg] || mappedFlags[arg] {
			return true
		}
		if optionTakesSeparateValue(arg) && i+1 < len(args) {
			i++
		}
	}
	return false
}
