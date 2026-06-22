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
		if IsTaskFlag(arg) || IsTask(arg) {
			priority = append(priority, arg)
			continue
		}
		normal = append(normal, arg)
		if OptionTakesSeparateValue(arg) && i+1 < len(prepared) {
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
		if ShortOptionTakesAttachedValue(arg[1]) {
			expanded = append(expanded, arg[:2], arg[2:])
			continue
		}
		for _, flag := range arg[1:] {
			expanded = append(expanded, "-"+string(flag))
		}
	}
	return expanded
}

func containsKnownTaskOrMappedFlag(args []string) bool {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if IsTask(arg) || IsTaskFlag(arg) {
			return true
		}
		if OptionTakesSeparateValue(arg) && i+1 < len(args) {
			i++
		}
	}
	return false
}
