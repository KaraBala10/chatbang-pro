package cli

import "testing"

func TestParse(t *testing.T) {
	tests := []struct {
		name string
		args []string
		base bool
		want Options
	}{
		{
			name: "defaults",
			args: []string{"chatbang-pro"},
			base: true,
			want: Options{Headless: true},
		},
		{
			name: "help",
			args: []string{"chatbang-pro", "--help"},
			base: true,
			want: Options{Headless: true, WantHelp: true},
		},
		{
			name: "config",
			args: []string{"chatbang-pro", "--config"},
			base: true,
			want: Options{Headless: true, WantConfig: true},
		},
		{
			name: "no headless overrides config default",
			args: []string{"chatbang-pro", "--no-headless"},
			base: true,
			want: Options{Headless: false},
		},
		{
			name: "headless from config base false",
			args: []string{"chatbang-pro", "--headless"},
			base: false,
			want: Options{Headless: true},
		},
		{
			name: "temporary chat",
			args: []string{"chatbang-pro", "--temp"},
			base: true,
			want: Options{Headless: true, TemporaryChat: true},
		},
		{
			name: "gpt flag",
			args: []string{"chatbang-pro", "-g", "g-abc123"},
			base: true,
			want: Options{Headless: true, CustomGPT: "g-abc123"},
		},
		{
			name: "gpt equals form",
			args: []string{"chatbang-pro", "--gpt=g-abc123"},
			base: true,
			want: Options{Headless: true, CustomGPT: "g-abc123"},
		},
		{
			name: "message flag",
			args: []string{"chatbang-pro", "-m", "hello"},
			base: true,
			want: Options{Headless: true, MessageFlag: true, Message: "hello"},
		},
		{
			name: "message equals form",
			args: []string{"chatbang-pro", "--message=hello"},
			base: true,
			want: Options{Headless: true, MessageFlag: true, Message: "hello"},
		},
		{
			name: "instances flag",
			args: []string{"chatbang-pro", "--instances"},
			base: true,
			want: Options{Headless: true, WantInstances: true},
		},
		{
			name: "combined flags",
			args: []string{"chatbang-pro", "--temp", "-g", "g-abc123", "-m", "hi"},
			base: true,
			want: Options{
				Headless:      true,
				TemporaryChat: true,
				CustomGPT:     "g-abc123",
				MessageFlag:   true,
				Message:       "hi",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Parse(tt.args, tt.base)
			if got != tt.want {
				t.Fatalf("Parse() = %+v, want %+v", got, tt.want)
			}
		})
	}
}

func TestIsExitCommand(t *testing.T) {
	for _, cmd := range []string{"exit", "EXIT", " quit ", "q", ":q", "/exit"} {
		if !IsExitCommand(cmd) {
			t.Fatalf("%q should be an exit command", cmd)
		}
	}
	if IsExitCommand("hello") {
		t.Fatal("hello should not be an exit command")
	}
}
