package server

import (
	"context"
	"net/http"
	"os/exec"
	"strconv"

	"github.com/gorilla/websocket"
)

const (
	indexEndpoint   = "/"
	initEndpoint    = "/init"
	pageEndpoint    = "/page"
	executeEndpoint = "/execute"

	terminalBufferSize = 1024
)

var (
	upgrader = websocket.Upgrader{
		ReadBufferSize:  terminalBufferSize,
		WriteBufferSize: terminalBufferSize,
		CheckOrigin: func(r *http.Request) bool {
			return true // Allow connections from any origin
		},
	}
)

func (m *DemoManager) indexHandler(w http.ResponseWriter, r *http.Request) {
	if err := m.stopCurrentCommand(); err != nil {
		m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
		return
	}

	m.setCommand(0)
	m.indexHTML().Render(w)
}

// TODO: improve this so that it is more seemless and doesn't cause any panics
func (m *DemoManager) initHandler(w http.ResponseWriter, r *http.Request) {
	m.logInfo(w, "attempting to upgrade HTTP to websocket")

	if m.ws != nil {
		m.logInfo(w, "closing existing websocket")
		if err := m.ws.Close(); err != nil {
			m.logError(w, "error closing existing websocket: %v", err.Error())
		}
	}

	var err error
	m.ws, err = upgrader.Upgrade(w, r, nil)
	if err != nil {
		m.logError(w, "error upgrading to websocket: %v", err.Error())
		return
	}

	m.logInfo(w, "upgraded HTTP connection to websocket")
}

func (m *DemoManager) incPageHandler(w http.ResponseWriter, r *http.Request) {
	prevCommand := m.getCommand()
	if err := m.stopCurrentCommand(); err != nil {
		m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
		return
	}

	m.incCommand()

	if currCommand := m.getCommand(); prevCommand != currCommand {
		m.contentDiv().Render(w)
	} else {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := m.termClear(); err != nil {
		m.logError(w, "error sending clear to websocket: %v", err.Error())
		return
	}
}

func (m *DemoManager) decPageHandler(w http.ResponseWriter, r *http.Request) {
	prevCommand := m.getCommand()
	if err := m.stopCurrentCommand(); err != nil {
		m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
		return
	}

	m.decCommand()

	if currCommand := m.getCommand(); prevCommand != currCommand {
		m.contentDiv().Render(w)
	} else {
		w.WriteHeader(http.StatusNoContent)
		return
	}

	if err := m.termClear(); err != nil {
		m.logError(w, "error sending clear to websocket: %v", err.Error())
		return
	}
}

func (m *DemoManager) setPageHandler(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		m.logError(w, "failed to parse form: %v", err.Error())
		return
	}

	slideIndex, err := strconv.Atoi(r.FormValue("slideIndex"))
	if err != nil {
		m.logError(w, "failed to parse slide index: %v", err.Error())
		return
	}

	prevCommand := m.getCommand()
	m.setCommand(int32(slideIndex))

	if currCommand := m.getCommand(); prevCommand != currCommand {
		if err := m.stopCurrentCommand(); err != nil {
			m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
			return
		}

		m.contentDiv().Render(w)
	}

	if err := m.termClear(); err != nil {
		m.logError(w, "error sending clear to websocket: %v", err.Error())
	}
}

func (m *DemoManager) executeCommandHandler(w http.ResponseWriter, r *http.Request) {
	if !m.isCommand() {
		return
	}

	if err := m.stopCurrentCommand(); err != nil {
		m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
		return
	}

	if err := m.termClear(); err != nil {
		m.logError(w, "error sending clear to websocket: %v", err.Error())
	}

	var cmd *exec.Cmd
	m.cmdContext, m.cancelCommand = context.WithCancel(context.Background())
	cmd = exec.CommandContext(m.cmdContext, "sh", "-c", m.cleanedCommand())
	stdoutPipe, err := cmd.StdoutPipe()
	if err != nil {
		m.logError(w, "error creating stdout pipe for command %v: %v", m.cleanedCommand(), err.Error())
		return
	}
	cmd.Stderr = cmd.Stdout
	m.cmd.Store(cmd)

	m.runningButton().Render(w)

	go func() {
		m.logInfo(w, "starting command '%v'", m.cleanedCommand())
		if err := m.executeCommand(stdoutPipe); err != nil {
			m.logError(w, "error executing command %v: %v", m.cleanedCommand(), err.Error())
		}
		return
	}()
}

func (m *DemoManager) executeStatusHandler(w http.ResponseWriter, r *http.Request) {
	if running := m.isCmdRunning(); !running {
		m.runningButton().Render(w)
	} else {
		w.WriteHeader(http.StatusNoContent)
	}
}

func (m *DemoManager) stopCommandHandler(w http.ResponseWriter, r *http.Request) {
	if err := m.stopCurrentCommand(); err != nil {
		m.logError(w, "error stopping command %v: %v", m.cleanedCommand(), err.Error())
		return
	}

	m.runningButton().Render(w)
}
