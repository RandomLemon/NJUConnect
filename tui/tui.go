package tui

import (
	"EasierConnect/core"
	"bufio"
	"errors"
	"fmt"
	"log"
	"os"
	"strconv"
	"strings"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type Status int

const (
	StatusDisconnected Status = iota
	StatusConnecting
	StatusConnected
	StatusError
)

type screen int

const (
	screenMenu screen = iota
	screenConnect
	screenDashboard
	screenSMS
	screenTOTP
	screenProfiles
	screenHelp
)

type logMsg string
type errMsg struct{ err error }
type authRequiredMsg struct {
	client   *core.EasyConnectClient
	authType string
}
type connectedMsg struct {
	ip     []byte
	client *core.EasyConnectClient
}

func (e errMsg) Error() string { return e.err.Error() }

type model struct {
	screen     screen
	prevScreen screen

	inputs []textinput.Model
	focus  int

	authInput textinput.Model

	pendingClient *core.EasyConnectClient
	pendingAuth   string

	client    *core.EasyConnectClient
	clientIP  string
	status    Status
	socksBind string

	logs     []string
	logPipeR *os.File

	spinner spinner.Model
	loading bool
	loadMsg string

	width  int
	height int
	ready  bool

	profiles         []Profile
	activeProfileIdx int

	lastError string
	quitting  bool
}

func initialModel() *model {
	inputs := make([]textinput.Model, 5)

	placeholders := []string{"vpn.example.com", "443", "username", "password", ":1080"}
	prompts := []string{"Server", "Port", "Username", "Password", "SOCKS5 Bind"}
	for i := range inputs {
		inputs[i] = textinput.New()
		inputs[i].Placeholder = placeholders[i]
		inputs[i].Prompt = prompts[i] + " > "
		inputs[i].CharLimit = 128
		inputs[i].Width = 40

		if i == 3 {
			inputs[i].EchoMode = textinput.EchoPassword
		}
		if i == 1 {
			inputs[i].SetValue("443")
		}
		if i == 4 {
			inputs[i].SetValue(":1080")
		}
	}
	inputs[0].Focus()

	authInput := textinput.New()
	authInput.Placeholder = "Enter code..."
	authInput.Prompt = "Code > "
	authInput.CharLimit = 32
	authInput.Width = 20

	s := spinner.New()
	s.Style = spinnerStyle

	cfg, _ := loadConfig()
	profiles := []Profile{}
	if cfg != nil {
		profiles = cfg.Profiles
	}

	return &model{
		screen:           screenMenu,
		inputs:           inputs,
		authInput:        authInput,
		spinner:          s,
		status:           StatusDisconnected,
		loadMsg:          "Connecting...",
		logs:             []string{},
		profiles:         profiles,
		activeProfileIdx: -1,
	}
}

func Start() {
	r, w, err := os.Pipe()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to create log pipe: %v\n", err)
		os.Exit(1)
	}
	log.SetOutput(w)

	logChan := make(chan string, 200)
	go func() {
		scanner := bufio.NewScanner(r)
		for scanner.Scan() {
			logChan <- scanner.Text()
		}
	}()

	m := initialModel()
	m.logPipeR = r

	p := tea.NewProgram(m, tea.WithAltScreen())

	go func() {
		for line := range logChan {
			p.Send(logMsg(line))
		}
	}()

	if _, err := p.Run(); err != nil {
		log.SetOutput(os.Stderr)
		fmt.Fprintf(os.Stderr, "Error running TUI: %v\n", err)
		os.Exit(1)
	}

	log.SetOutput(os.Stderr)
	w.Close()
}

func (m model) Init() tea.Cmd {
	return tea.Batch(
		spinner.Tick,
		tea.WindowSize(),
	)
}

func (m model) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if m.quitting {
		return m, tea.Quit
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width = msg.Width
		m.height = msg.Height
		m.ready = true
		return m, nil

	case tea.KeyMsg:
		if msg.Type == tea.KeyCtrlC {
			m.quitting = true
			return m, tea.Quit
		}
		return m.handleKeyMsg(msg)

	case logMsg:
		m.logs = append(m.logs, string(msg))
		if len(m.logs) > 1000 {
			m.logs = m.logs[len(m.logs)-500:]
		}
		return m, nil

	case errMsg:
		m.loading = false
		m.lastError = msg.err.Error()
		m.status = StatusError
		return m, nil

	case authRequiredMsg:
		m.loading = false
		m.pendingClient = msg.client
		m.pendingAuth = msg.authType
		m.prevScreen = m.screen
		if msg.authType == "sms" {
			m.screen = screenSMS
		} else {
			m.screen = screenTOTP
		}
		m.authInput.Focus()
		m.authInput.SetValue("")
		return m, nil

	case connectedMsg:
		m.loading = false
		m.client = msg.client
		m.clientIP = fmt.Sprintf("%d.%d.%d.%d", msg.ip[0], msg.ip[1], msg.ip[2], msg.ip[3])
		m.status = StatusConnected
		m.screen = screenDashboard
		m.saveProfile()
		saveConfig(&Config{Profiles: m.profiles})
		go func(c *core.EasyConnectClient) {
			c.ServeSocks5(core.SocksBind, core.DebugDump)
		}(msg.client)
		return m, nil

	case spinner.TickMsg:
		var cmd tea.Cmd
		m.spinner, cmd = m.spinner.Update(msg)
		return m, cmd

	default:
		return m, nil
	}
}

func (m *model) handleKeyMsg(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch m.screen {
	case screenMenu:
		return m.handleMenuKey(msg)
	case screenConnect:
		return m.handleConnectKey(msg)
	case screenSMS, screenTOTP:
		return m.handleAuthKey(msg)
	case screenDashboard:
		return m.handleDashboardKey(msg)
	case screenProfiles:
		return m.handleProfilesKey(msg)
	case screenHelp:
		return m.handleHelpKey(msg)
	default:
		return m, nil
	}
}

func (m *model) menuAction(key string) (screen, bool) {
	hasProfiles := len(m.profiles) > 0
	switch {
	case hasProfiles && key == "1":
		return screenProfiles, true
	case (hasProfiles && key == "2") || (!hasProfiles && key == "1"):
		return screenConnect, true
	case (hasProfiles && key == "3") || (!hasProfiles && key == "2"):
		return screenHelp, true
	}
	return 0, false
}

func (m *model) handleMenuKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.quitting = true
		return m, tea.Quit
	default:
		s, ok := m.menuAction(msg.String())
		if !ok {
			return m, nil
		}
		switch s {
		case screenConnect:
			m.screen = screenConnect
			m.inputs[0].Focus()
			m.focus = 0
			m.lastError = ""
		case screenProfiles:
			m.screen = screenProfiles
		case screenHelp:
			m.prevScreen = screenMenu
			m.screen = screenHelp
		}
		return m, nil
	}
}

func (m *model) handleConnectKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "tab":
		m.focus = (m.focus + 1) % len(m.inputs)
		m.updateInputs()
		return m, nil
	case "shift+tab":
		m.focus--
		if m.focus < 0 {
			m.focus = len(m.inputs) - 1
		}
		m.updateInputs()
		return m, nil
	case "enter":
		if m.focus == len(m.inputs)-1 {
			if m.loading {
				return m, nil
			}
			m.socksBind = m.inputs[4].Value()
			core.SocksBind = m.socksBind
			m.loading = true
			return m, m.loginCmd()
		}
		m.focus++
		m.updateInputs()
		return m, nil
	case "esc":
		if m.loading {
			return m, nil
		}
		m.screen = screenMenu
		return m, nil
	default:
		var cmd tea.Cmd
		m.inputs[m.focus], cmd = m.inputs[m.focus].Update(msg)
		return m, cmd
	}
}

func (m *model) handleAuthKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "enter":
		code := m.authInput.Value()
		if code == "" {
			return m, nil
		}
		m.loading = true
		var cmd tea.Cmd
		if m.pendingAuth == "sms" {
			cmd = m.smsCmd(code)
		} else {
			cmd = m.totpCmd(code)
		}
		return m, cmd
	case "esc":
		m.screen = m.prevScreen
		m.pendingClient = nil
		m.pendingAuth = ""
		m.loading = false
		m.lastError = ""
		return m, nil
	default:
		var cmd tea.Cmd
		m.authInput, cmd = m.authInput.Update(msg)
		return m, cmd
	}
}

func (m *model) handleDashboardKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.quitting = true
		return m, tea.Quit
	default:
		return m, nil
	}
}

func (m *model) handleProfilesKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		return m, nil
	default:
		n, err := strconv.Atoi(msg.String())
		if err != nil || n < 1 || n > len(m.profiles) {
			return m, nil
		}
		m.loadProfile(n - 1)
		m.screen = screenConnect
		m.focus = 3
		m.inputs[3].Focus()
		m.lastError = ""
		return m, nil
	}
}

func (m *model) handleHelpKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	switch msg.String() {
	case "esc":
		m.screen = screenMenu
		return m, nil
	default:
		return m, nil
	}
}

func (m *model) updateInputs() {
	for i := range m.inputs {
		if i == m.focus {
			m.inputs[i].Focus()
		} else {
			m.inputs[i].Blur()
		}
	}
}

func (m *model) loginCmd() tea.Cmd {
	return func() tea.Msg {
		host := m.inputs[0].Value()
		portStr := m.inputs[1].Value()
		if portStr == "" {
			portStr = "443"
		}
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return errMsg{err: fmt.Errorf("invalid port: %s (must be a number)", portStr)}
		}
		username := m.inputs[2].Value()
		password := m.inputs[3].Value()

		if host == "" || username == "" || password == "" {
			return errMsg{err: fmt.Errorf("server, username, and password are required")}
		}

		core.SocksBind = m.socksBind
		server := fmt.Sprintf("%s:%d", host, port)
		client := core.NewEasyConnectClient(server)

		ip, err := client.Login(username, password)
		if errors.Is(err, core.ERR_NEXT_AUTH_SMS) {
			return authRequiredMsg{client: client, authType: "sms"}
		}
		if errors.Is(err, core.ERR_NEXT_AUTH_TOTP) {
			return authRequiredMsg{client: client, authType: "totp"}
		}
		if err != nil {
			return errMsg{err: err}
		}

		return connectedMsg{ip: ip, client: client}
	}
}

func (m *model) smsCmd(code string) tea.Cmd {
	return func() tea.Msg {
		ip, err := m.pendingClient.AuthSMSCode(code)
		if err != nil {
			return errMsg{err: err}
		}
		return connectedMsg{ip: ip, client: m.pendingClient}
	}
}

func (m *model) totpCmd(code string) tea.Cmd {
	return func() tea.Msg {
		ip, err := m.pendingClient.AuthTOTP(code)
		if err != nil {
			return errMsg{err: err}
		}
		return connectedMsg{ip: ip, client: m.pendingClient}
	}
}

func (m model) View() string {
	if !m.ready {
		return "Initializing..."
	}

	content := ""
	switch m.screen {
	case screenMenu:
		content = m.menuView()
	case screenConnect:
		content = m.connectView()
	case screenDashboard:
		content = m.dashboardView()
	case screenSMS:
		content = m.authView("SMS Code Required")
	case screenTOTP:
		content = m.authView("TOTP Code Required")
	case screenProfiles:
		content = m.profilesView()
	case screenHelp:
		content = m.helpView()
	}

	return appStyle.Render(content)
}

func (m model) menuView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("EasierConnect TUI"))
	b.WriteString("\n\n")

	hasProfiles := len(m.profiles) > 0
	if hasProfiles {
		b.WriteString(dimStyle.Render("1.") + " Account Profiles\n")
		b.WriteString(dimStyle.Render("2.") + " Account Login\n")
		b.WriteString(dimStyle.Render("3.") + " Help\n")
	} else {
		b.WriteString(dimStyle.Render("1.") + " Account Login\n")
		b.WriteString(dimStyle.Render("2.") + " Help\n")
	}
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Esc") + ": Quit\n")
	b.WriteString("\n")
	if hasProfiles {
		b.WriteString(infoStyle.Render("Select an option (1-3)"))
	} else {
		b.WriteString(infoStyle.Render("Select an option (1-2)"))
	}

	return b.String()
}

func (m model) connectView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("VPN Connection Setup"))

	server := m.inputs[0].Value()
	username := m.inputs[2].Value()
	matched := false
	for _, p := range m.profiles {
		if p.Server == server && p.Username == username {
			b.WriteString("\n")
			b.WriteString(infoStyle.Render("Profile: " + p.Name))
			matched = true
			break
		}
	}
	if !matched && server != "" {
		b.WriteString("\n")
		b.WriteString(dimStyle.Render("Profile: (new — will be saved on connect)"))
	}

	b.WriteString("\n\n")

	for i := range m.inputs {
		b.WriteString(m.inputs[i].View())
		b.WriteString("\n")
	}

	if m.loading {
		b.WriteString("\n")
		b.WriteString(m.spinner.View() + " " + m.loadMsg + "\n")
	}

	if m.lastError != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.lastError))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Enter: next field  |  Tab: next  |  Shift+Tab: prev  |  Esc: back"))

	return b.String()
}

func (m model) dashboardView() string {
	var b strings.Builder

	dot := statusDotConnected.Render()
	b.WriteString(dot + " " + successStyle.Render("Connected"))
	b.WriteString("\n\n")

	b.WriteString(labelStyle.Render("Assigned IP:"))
	b.WriteString(" " + m.clientIP + "\n")

	b.WriteString(labelStyle.Render("SOCKS5:"))
	b.WriteString(" " + m.socksBind + "\n")

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("── Log ─────────────────────"))
	b.WriteString("\n")

	maxLogs := m.height - 10
	if maxLogs < 5 {
		maxLogs = 5
	}
	start := 0
	if len(m.logs) > maxLogs {
		start = len(m.logs) - maxLogs
	}
	for _, line := range m.logs[start:] {
		b.WriteString(logLineStyle.Render(line))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Esc: quit"))

	return b.String()
}

func (m model) authView(title string) string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(title))
	b.WriteString("\n\n")

	b.WriteString(m.authInput.View())
	b.WriteString("\n")

	if m.lastError != "" {
		b.WriteString("\n")
		b.WriteString(errorStyle.Render("Error: " + m.lastError))
		b.WriteString("\n")
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Enter: submit  |  Esc: cancel"))

	return b.String()
}

func (m model) profilesView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Saved Profiles"))
	b.WriteString("\n\n")

	if len(m.profiles) == 0 {
		b.WriteString(infoStyle.Render("No saved profiles yet."))
		b.WriteString("\n\n")
		b.WriteString(dimStyle.Render("Profiles are automatically saved after a successful connection."))
		b.WriteString("\n")
	} else {
		for i, p := range m.profiles {
			idx := dimStyle.Render(fmt.Sprintf("%d.", i+1))
			b.WriteString(fmt.Sprintf("%s %s\n", idx, p.Name))
			b.WriteString(dimStyle.Render(fmt.Sprintf("   %s / %s\n", p.Server, p.Username)))
		}
	}

	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Press number to load profile"))
	b.WriteString("\n")
	b.WriteString(dimStyle.Render("Esc: back"))

	return b.String()
}

func (m model) helpView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render("Help"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("About"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  EasierConnect"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("Menu"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  1  Connect to a VPN server with credentials"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  2  Browse saved connection profiles"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  3  Show this help"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Esc  Quit"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Ctrl+C  Quit (any screen)"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("Profiles"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Profiles are saved automatically on successful connection."))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Press the profile number to load it into the connect form."))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Config file: ~/.config/easierconnect/profiles.json"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("Connect Form"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Tab       Move to next field"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Shift+Tab Move to previous field"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Enter     Next field / Connect"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Esc       Back to menu"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("Dashboard"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Esc  Quit"))
	b.WriteString("\n\n")

	b.WriteString(helpKeyStyle.Render("Auth Screens"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Enter     Submit code"))
	b.WriteString("\n")
	b.WriteString(helpDescStyle.Render("  Esc       Cancel"))
	b.WriteString("\n\n")

	b.WriteString(dimStyle.Render("Esc: back"))

	return b.String()
}
