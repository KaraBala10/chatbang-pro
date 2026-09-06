package cli

import "strings"

// Options holds parsed command-line flags.
type Options struct {
	WantConfig     bool
	WantHelp       bool
	WantInstances  bool
	Headless       bool
	TemporaryChat  bool
	CustomGPT      string
	Message        string
	MessageFlag    bool
}

func flagValue(args []string, i int) (string, int, bool) {
	if i+1 >= len(args) {
		return "", i, false
	}
	return args[i+1], i + 1, true
}

func setCustomGPT(opts *Options, value string) {
	opts.CustomGPT = strings.TrimSpace(value)
}

// Parse applies flags and modes. --config takes precedence over --help.
func Parse(args []string, headless bool) Options {
	opts := Options{Headless: headless}
	for i := 1; i < len(args); i++ {
		arg := args[i]
		switch arg {
		case "--config":
			opts.WantConfig = true
		case "--help", "-h":
			opts.WantHelp = true
		case "--instances":
			opts.WantInstances = true
		case "--headless":
			opts.Headless = true
		case "--no-headless":
			opts.Headless = false
		case "--temporary-chat", "--temp":
			opts.TemporaryChat = true
		case "--gpt", "--custom-gpt", "-g":
			value, next, ok := flagValue(args, i)
			if !ok {
				continue
			}
			setCustomGPT(&opts, value)
			i = next
		case "--message", "-m":
			opts.MessageFlag = true
			value, next, ok := flagValue(args, i)
			if ok {
				opts.Message = value
				i = next
			}
		default:
			if value, ok := strings.CutPrefix(arg, "--gpt="); ok {
				setCustomGPT(&opts, value)
			} else if value, ok := strings.CutPrefix(arg, "--custom-gpt="); ok {
				setCustomGPT(&opts, value)
			} else if value, ok := strings.CutPrefix(arg, "--message="); ok {
				opts.MessageFlag = true
				opts.Message = value
			}
		}
	}
	return opts
}

// IsExitCommand reports whether the user typed a quit command.
func IsExitCommand(prompt string) bool {
	switch strings.ToLower(strings.TrimSpace(prompt)) {
	case "exit", "quit", "q", ":q", "/exit", "/quit":
		return true
	default:
		return false
	}
}
