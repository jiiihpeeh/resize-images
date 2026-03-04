package setup

import (
	"fmt"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

var (
	focusedStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("205"))
	blurredStyle = lipgloss.NewStyle().Foreground(lipgloss.Color("240"))
	cursorStyle  = focusedStyle.Copy()
	noStyle      = lipgloss.NewStyle()
	helpStyle    = blurredStyle.Copy()
)

type model struct {
	focusIndex int
	inputs     []textinput.Model
}

func initialModel() model {
	m := model{
		inputs: make([]textinput.Model, 8),
	}

	var t textinput.Model
	for i := range m.inputs {
		t = textinput.New()
		t.Cursor.Style = cursorStyle
		t.CharLimit = 128

		switch i {
		case 0:
			t.Placeholder = "3000"
			t.SetValue("3000")
			t.Prompt = "Server Port: "
			t.Focus()
			t.PromptStyle = focusedStyle
			t.TextStyle = focusedStyle
		case 1:
			t.Placeholder = "secret"
			t.SetValue("secret")
			t.Prompt = "JWT Secret: "
		case 2:
			t.Placeholder = "admin"
			t.SetValue("admin")
			t.Prompt = "Admin Username: "
		case 3:
			t.Placeholder = "password"
			t.SetValue("admin")
			t.Prompt = "Admin Password: "
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		case 4:
			t.Placeholder = "example.com,images.com"
			t.Prompt = "Allowed Hosts (empty=all): "
		case 5:
			t.Placeholder = "2"
			t.SetValue("2")
			t.Prompt = "Max Concurrent Resizes: "
		case 6:
			t.Placeholder = "60"
			t.SetValue("60")
			t.Prompt = "Requests Per Minute: "
		case 7:
			t.Placeholder = "secret-key"
			t.Prompt = "Registration Secret: "
			t.EchoMode = textinput.EchoPassword
			t.EchoCharacter = '•'
		}

		m.inputs[i] = t
	}

	return m
}

func (m model) Init() tea.Cmd {
	return textinput.Blink
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyMsg:
		switch msg.String() {
		case "ctrl+c", "esc":
			return m, tea.Quit

		// Set focus to next input
		case "tab", "shift+tab", "enter", "up", "down":
			s := msg.String()

			// Did the user press enter while the submit button was focused?
			if s == "enter" && m.focusIndex == len(m.inputs) {
				port := m.inputs[0].Value()
				secret := m.inputs[1].Value()
				user := m.inputs[2].Value()
				pass := m.inputs[3].Value()
				hosts := m.inputs[4].Value()
				concurrent := m.inputs[5].Value()
				rate := m.inputs[6].Value()
				regSecret := m.inputs[7].Value()

				if err := saveConfig(port, secret, user, pass, hosts, concurrent, rate, regSecret); err != nil {
					fmt.Printf("Error saving config: %v\n", err)
					return m, tea.Quit
				}
				fmt.Println("Configuration saved to .env")
				return m, tea.Quit
			}

			// Cycle indexes
			if s == "up" || s == "shift+tab" {
				m.focusIndex--
			} else {
				m.focusIndex++
			}

			if m.focusIndex > len(m.inputs) {
				m.focusIndex = 0
			} else if m.focusIndex < 0 {
				m.focusIndex = len(m.inputs)
			}

			cmds := make([]tea.Cmd, len(m.inputs))
			for i := 0; i <= len(m.inputs)-1; i++ {
				if i == m.focusIndex {
					// Set focused state
					cmds[i] = m.inputs[i].Focus()
					m.inputs[i].PromptStyle = focusedStyle
					m.inputs[i].TextStyle = focusedStyle
				} else {
					// Remove focused state
					m.inputs[i].Blur()
					m.inputs[i].PromptStyle = noStyle
					m.inputs[i].TextStyle = noStyle
				}
			}

			return m, tea.Batch(cmds...)
		}
	}

	// Handle character input and blinking
	cmd := m.updateInputs(msg)

	return m, cmd
}

func (m *model) updateInputs(msg tea.Msg) tea.Cmd {
	cmds := make([]tea.Cmd, len(m.inputs))

	// Only update the focused input
	if m.focusIndex < len(m.inputs) {
		m.inputs[m.focusIndex], cmds[m.focusIndex] = m.inputs[m.focusIndex].Update(msg)
	}

	return tea.Batch(cmds...)
}

func (m model) View() string {
	var b strings.Builder

	b.WriteString("\n  First Time Setup\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		if i < len(m.inputs)-1 {
			b.WriteRune('\n')
		}
		b.WriteRune('\n')
	}

	button := "\n  [ Submit ]\n\n"
	if m.focusIndex == len(m.inputs) {
		button = focusedStyle.Render("\n  [ Submit ]\n\n")
	}
	b.WriteString(button)

	b.WriteString(helpStyle.Render("  (esc to quit)"))
	b.WriteString("\n")

	return b.String()
}

func Run() {
	if _, err := tea.NewProgram(initialModel()).Run(); err != nil {
		fmt.Printf("could not start program: %s\n", err)
		os.Exit(1)
	}
}

func saveConfig(port, secret, user, pass, hosts, concurrent, rate, regSecret string) error {
	content := fmt.Sprintf(`# Server Configuration
SERVER_HOST=0.0.0.0
SERVER_PORT=%s

# JWT Configuration
JWT_SECRET=%q
JWT_EXPIRY=24h
JWT_REFRESH_EXPIRY=7d

# Admin Credentials
ADMIN_USER=%s
ADMIN_PASSWORD=%q

# Registration Security
REGISTRATION_SECRET=%q

# Image Processing Configuration
MAX_WIDTH=4096
MAX_HEIGHT=4096
IMAGE_TIMEOUT=30s

# Allowed hosts for image URLs (comma-separated, empty = allow all)
ALLOWED_HOSTS=%s

# Quality Settings (1-100)
QUALITY_JPEG=80
QUALITY_PNG=80
QUALITY_WEBP=80
QUALITY_AVIF=60
QUALITY_JXL=75

# Constraints for computationally expensive formats
AVIF_MAX_RESOLUTION=2048
JXL_MAX_RESOLUTION=1920
AVIF_MAX_PIXELS=2500000
JXL_MAX_PIXELS=2000000

# Enable/disable formats
ENABLE_AVIF=true
ENABLE_JXL=true

# Rate limiting
MAX_CONCURRENT=%s
REQUESTS_PER_MIN=%s
`, port, secret, user, pass, regSecret, hosts, concurrent, rate)
	return os.WriteFile(".env", []byte(content), 0644)
}
