package internal

import (
	"bufio"
	"context"
	"fmt"
	"image"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/alecthomas/chroma/v2/lexers"
	"github.com/charmbracelet/lipgloss"
	"github.com/charmbracelet/x/exp/term/ansi"
	"github.com/yorukot/ansichroma"
	"github.com/yorukot/superfile/src/config/icon"
	filepreview "github.com/yorukot/superfile/src/pkg/file_preview"
)

func (m *model) sidebarRender() string {
	if Config.SidebarWidth == 0 {
		return ""
	}
	slog.Debug("Rendering sidebar.", "cursor", m.sidebarModel.cursor,
		"renderIndex", m.sidebarModel.renderIndex, "dirs count", len(m.sidebarModel.directories),
		"sidebar focused", m.focusPanel == sidebarFocus)

	s := sideBarSuperfileTitle + "\n"

	if m.sidebarModel.searchBar.Focused() || m.sidebarModel.searchBar.Value() != "" || m.focusPanel == sidebarFocus {
		m.sidebarModel.searchBar.Placeholder = "(" + hotkeys.SearchBar[0] + ")" + " Search"
		s += "\n" + ansi.Truncate(m.sidebarModel.searchBar.View(), Config.SidebarWidth-2, "...")
	}

	if m.sidebarModel.noActualDir() {
		s += "\n" + sideBarNoneText
		return sideBarBorderStyle(m.mainPanelHeight, m.focusPanel).Render(s)
	}

	s += m.sidebarModel.directoriesRender(m.mainPanelHeight,
		m.fileModel.filePanels[m.filePanelFocusIndex].location, m.focusPanel == sidebarFocus)

	return sideBarBorderStyle(m.mainPanelHeight, m.focusPanel).Render(s)
}

func (s *sidebarModel) directoriesRender(mainPanelHeight int, curFilePanelFileLocation string, sideBarFocussed bool) string {

	// Cursor should always point to a valid directory at this point
	if s.isCursorInvalid() {
		slog.Error("Unexpected situation in sideBar Model. "+
			"Cursor is at invalid postion, while there are valide directories", "cursor", s.cursor,
			"directory count", len(s.directories))
		return ""
	}

	res := ""
	totalHeight := sideBarInitialHeight
	for i := s.renderIndex; i < len(s.directories); i++ {
		if totalHeight+s.directories[i].requiredHeight() > mainPanelHeight {
			break
		}
		res += "\n"

		totalHeight += s.directories[i].requiredHeight()

		if s.directories[i] == pinnedDividerDir {
			res += "\n" + sideBarPinnedDivider
		} else if s.directories[i] == diskDividerDir {
			res += "\n" + sideBarDisksDivider
		} else {
			cursor := " "
			if s.cursor == i && sideBarFocussed && !s.searchBar.Focused() {
				cursor = icon.Cursor
			}
			if s.renaming && s.cursor == i {
				res += s.rename.View()
			} else {
				renderStyle := sidebarStyle
				if s.directories[i].location == curFilePanelFileLocation {
					renderStyle = sidebarSelectedStyle
				}
				res += filePanelCursorStyle.Render(cursor+" ") +
					renderStyle.Render(truncateText(s.directories[i].name, Config.SidebarWidth-2, "..."))
			}
		}
	}
	return res
}

// This also modifies the m.fileModel.filePanels
func (m *model) filePanelRender() string {
	// file panel
	f := make([]string, 10)
	for i, filePanel := range m.fileModel.filePanels {

		// check if cursor or render out of range
		if filePanel.cursor > len(filePanel.element)-1 {
			filePanel.cursor = 0
			filePanel.render = 0
		}
		m.fileModel.filePanels[i] = filePanel

		f[i] += filePanelTopDirectoryIconStyle.Render(" "+icon.Directory+icon.Space) + filePanelTopPathStyle.Render(truncateTextBeginning(filePanel.location, m.fileModel.width-4, "...")) + "\n"
		filePanelWidth := 0
		footerBorderWidth := 0

		if (m.fullWidth-Config.SidebarWidth-(4+(len(m.fileModel.filePanels)-1)*2))%len(m.fileModel.filePanels) != 0 && i == len(m.fileModel.filePanels)-1 {
			if m.fileModel.filePreview.open {
				filePanelWidth = m.fileModel.width
			} else {
				filePanelWidth = (m.fileModel.width + (m.fullWidth-Config.SidebarWidth-(4+(len(m.fileModel.filePanels)-1)*2))%len(m.fileModel.filePanels))
			}
			footerBorderWidth = m.fileModel.width + 15
		} else {
			filePanelWidth = m.fileModel.width
			footerBorderWidth = m.fileModel.width + 15
		}

		sortDirectionString := ""
		if filePanel.sortOptions.data.reversed {
			if Config.Nerdfont {
				sortDirectionString = icon.SortDesc
			} else {
				sortDirectionString = "D"
			}
		} else {
			if Config.Nerdfont {
				sortDirectionString = icon.SortAsc
			} else {
				sortDirectionString = "A"
			}
		}
		sortTypeString := ""
		if filePanelWidth < 23 {
			sortTypeString = sortDirectionString
		} else {
			if filePanel.sortOptions.data.options[filePanel.sortOptions.data.selected] == "Date Modified" {
				sortTypeString = sortDirectionString + icon.Space + "Date"
			} else {
				sortTypeString = sortDirectionString + icon.Space + filePanel.sortOptions.data.options[filePanel.sortOptions.data.selected]
			}
		}

		panelModeString := ""
		if filePanelWidth < 23 {
			if filePanel.panelMode == browserMode {
				if Config.Nerdfont {
					panelModeString = icon.Browser
				} else {
					panelModeString = "B"
				}
			} else if filePanel.panelMode == selectMode {
				if Config.Nerdfont {
					panelModeString = icon.Select
				} else {
					panelModeString = "S"
				}
			}
		} else {
			if filePanel.panelMode == browserMode {
				panelModeString = icon.Browser + icon.Space + "Browser"
			} else if filePanel.panelMode == selectMode {
				panelModeString = icon.Select + icon.Space + "Select"
			}
		}

		f[i] += filePanelDividerStyle(filePanel.focusType).Render(strings.Repeat(Config.BorderTop, filePanelWidth)) + "\n"
		f[i] += " " + filePanel.searchBar.View() + "\n"
		if len(filePanel.element) == 0 {
			f[i] += filePanelStyle.Render(" " + icon.Error + "  No such file or directory")
			bottomBorder := generateFooterBorder(fmt.Sprintf("%s%s%s%s%s", sortTypeString, bottomMiddleBorderSplit, panelModeString, bottomMiddleBorderSplit, "0/0"), footerBorderWidth)
			f[i] = filePanelBorderStyle(m.mainPanelHeight, filePanelWidth, filePanel.focusType, bottomBorder).Render(f[i])
		} else {
			for h := filePanel.render; h < filePanel.render+panelElementHeight(m.mainPanelHeight) && h < len(filePanel.element); h++ {
				endl := "\n"
				if h == filePanel.render+panelElementHeight(m.mainPanelHeight)-1 || h == len(filePanel.element)-1 {
					endl = ""
				}
				cursor := " "
				// Check if the cursor needs to be displayed, if the user is using the search bar, the cursor is not displayed
				if h == filePanel.cursor && !filePanel.searchBar.Focused() {
					cursor = icon.Cursor
				}
				isItemSelected := arrayContains(filePanel.selected, filePanel.element[h].location)
				if filePanel.renaming && h == filePanel.cursor {
					f[i] += filePanel.rename.View() + endl
				} else {
					_, err := os.ReadDir(filePanel.element[h].location)
					f[i] += filePanelCursorStyle.Render(cursor+" ") + prettierName(filePanel.element[h].name, m.fileModel.width-5, filePanel.element[h].directory || (err == nil), isItemSelected, filePanelBGColor) + endl
				}
			}
			cursorPosition := strconv.Itoa(filePanel.cursor + 1)
			totalElement := strconv.Itoa(len(filePanel.element))

			bottomBorder := generateFooterBorder(fmt.Sprintf("%s%s%s%s%s/%s", sortTypeString, bottomMiddleBorderSplit, panelModeString, bottomMiddleBorderSplit, cursorPosition, totalElement), footerBorderWidth)
			f[i] = filePanelBorderStyle(m.mainPanelHeight, filePanelWidth, filePanel.focusType, bottomBorder).Render(f[i])
		}
	}

	// file panel render together
	filePanelRender := ""
	for _, f := range f {
		filePanelRender = lipgloss.JoinHorizontal(lipgloss.Top, filePanelRender, f)
	}
	return filePanelRender
}
func (m *model) processBarRender() string {
	if !m.processBarModel.isValid(m.footerHeight) {
		slog.Error("processBar in invalid state", "render", m.processBarModel.render,
			"cursor", m.processBarModel.cursor, "footerHeight", m.footerHeight)
	}

	if len(m.processBarModel.processList) == 0 {
		processRender := "\n " + icon.Error + "  No processes running"
		return m.wrapProcessBardBorder(processRender)
	}

	// save process in the array and sort the process by finished or not,
	// completion percetage, or finish time
	// Todo : This is very inefficient and can be improved.
	// The whole design needs to be changed so that we dont need to recreate the slice
	// and sort on each render. Idea : Maintain two slices - completed, ongoing
	// Processes should be added / removed to the slice on correct time, and we dont
	// need to redo slice formation and sorting on each render.
	var processes []process
	for _, p := range m.processBarModel.process {
		processes = append(processes, p)
	}
	// sort by the process
	sort.Slice(processes, func(i, j int) bool {
		doneI := (processes[i].state == successful)
		doneJ := (processes[j].state == successful)

		// sort by done or not
		if doneI != doneJ {
			return !doneI
		}

		// if both not done
		if !doneI {
			completionI := float64(processes[i].done) / float64(processes[i].total)
			completionJ := float64(processes[j].done) / float64(processes[j].total)
			return completionI < completionJ // Those who finish first will be ranked later.
		}

		// if both done sort by the doneTime
		return processes[j].doneTime.Before(processes[i].doneTime)
	})

	// render
	processRender := ""
	renderedHeight := 0

	for i := m.processBarModel.render; i < len(processes); i++ {
		// Cant render any more processes

		// We allow rendering of a process if we have at least 2 lines left
		// Then we dont add a separator newline
		if m.footerHeight < renderedHeight+2 {
			break
		}
		renderedHeight += 3
		endSeparator := "\n\n"

		// Last process, but can render full in three lines
		// Although there is no next process, so dont add extra newline
		if m.footerHeight == renderedHeight {
			endSeparator = "\n"
		}

		// Cant add newline after last process. Only have two lines
		if m.footerHeight < renderedHeight {
			endSeparator = ""
			renderedHeight--
		}

		process := processes[i]
		process.progress.Width = footerWidth(m.fullWidth) - 3
		symbol := ""
		cursor := ""
		if i == m.processBarModel.cursor {
			cursor = footerCursorStyle.Render("┃ ")
		} else {
			cursor = footerCursorStyle.Render("  ")
		}
		switch process.state {
		case failure:
			symbol = processErrorStyle.Render(icon.Warn)
		case successful:
			symbol = processSuccessfulStyle.Render(icon.Done)
		case inOperation:
			symbol = processInOperationStyle.Render(icon.InOperation)
		case cancel:
			symbol = processCancelStyle.Render(icon.Error)
		}

		processRender += cursor + footerStyle.Render(truncateText(process.name, footerWidth(m.fullWidth)-7, "...")+" ") + symbol + "\n"

		processRender += cursor + process.progress.ViewAs(float64(process.done)/float64(process.total)) + endSeparator
	}

	return m.wrapProcessBardBorder(processRender)
}

func (m *model) wrapProcessBardBorder(processRender string) string {
	courseNumber := 0
	if len(m.processBarModel.processList) == 0 {
		courseNumber = 0
	} else {
		courseNumber = m.processBarModel.cursor + 1
	}
	bottomBorder := generateFooterBorder(fmt.Sprintf("%s/%s", strconv.Itoa(courseNumber), strconv.Itoa(len(m.processBarModel.processList))), footerWidth(m.fullWidth)-3)
	processRender = procsssBarBorder(m.footerHeight, footerWidth(m.fullWidth), bottomBorder, m.focusPanel).Render(processRender)

	return processRender
}

// This updates m.fileMetaData
func (m *model) metadataRender() string {
	// process bar
	metaDataBar := ""
	if len(m.fileMetaData.metaData) == 0 && len(m.fileModel.filePanels[m.filePanelFocusIndex].element) > 0 && !m.fileModel.renaming {
		m.fileMetaData.metaData = append(m.fileMetaData.metaData, [2]string{"", ""})
		m.fileMetaData.metaData = append(m.fileMetaData.metaData, [2]string{" " + icon.InOperation + "  Loading metadata...", ""})
		go func() {
			m.returnMetaData()
		}()
	}
	maxKeyLength := 0
	// Todo : The whole intention of this is to get the comparisonFields come before
	// other fields. Sorting like this is a bad way of achieving that. This can be improved
	sort.Slice(m.fileMetaData.metaData, func(i, j int) bool {
		// Initialising a new slice in each check by sort functions is too ineffinceint.
		// Todo : Fix it
		comparisonFields := []string{"FileName", "FileSize", "FolderName", "FolderSize", "FileModifyDate", "FileAccessDate"}

		for _, field := range comparisonFields {
			if m.fileMetaData.metaData[i][0] == field {
				return true
			} else if m.fileMetaData.metaData[j][0] == field {
				return false
			}
		}

		// Default comparison
		return m.fileMetaData.metaData[i][0] < m.fileMetaData.metaData[j][0]
	})
	for _, data := range m.fileMetaData.metaData {
		if len(data[0]) > maxKeyLength {
			maxKeyLength = len(data[0])
		}
	}

	// Todo : Too much calculations that are not in a fuctions, are not
	// unit tested, and have no proper explanation. This makes it
	// very hard to maintain and add any changes
	sprintfLength := maxKeyLength + 1
	valueLength := footerWidth(m.fullWidth) - maxKeyLength - 2
	if valueLength < footerWidth(m.fullWidth)/2 {
		valueLength = footerWidth(m.fullWidth)/2 - 2
		sprintfLength = valueLength
	}

	imax := min(m.footerHeight+m.fileMetaData.renderIndex, len(m.fileMetaData.metaData))
	for i := m.fileMetaData.renderIndex; i < imax; i++ {
		// Newline separator before all entries except first
		if i != m.fileMetaData.renderIndex {
			metaDataBar += "\n"
		}
		data := truncateMiddleText(m.fileMetaData.metaData[i][1], valueLength, "...")
		metadataName := m.fileMetaData.metaData[i][0]
		if footerWidth(m.fullWidth)-maxKeyLength-3 < footerWidth(m.fullWidth)/2 {
			metadataName = truncateMiddleText(m.fileMetaData.metaData[i][0], valueLength, "...")
		}
		metaDataBar += fmt.Sprintf("%-*s %s", sprintfLength, metadataName, data)

	}
	bottomBorder := generateFooterBorder(fmt.Sprintf("%s/%s", strconv.Itoa(m.fileMetaData.renderIndex+1), strconv.Itoa(len(m.fileMetaData.metaData))), footerWidth(m.fullWidth)-3)
	metaDataBar = metadataBorder(m.footerHeight, footerWidth(m.fullWidth), bottomBorder, m.focusPanel).Render(metaDataBar)

	return metaDataBar
}

func (m *model) clipboardRender() string {

	// render
	clipboardRender := ""
	if len(m.copyItems.items) == 0 {
		clipboardRender += "\n " + icon.Error + "  No content in clipboard"
	} else {
		for i := 0; i < len(m.copyItems.items) && i < m.footerHeight; i++ {
			// Newline separator before all entries except first
			if i != 0 {
				clipboardRender += "\n"
			}
			if i == m.footerHeight-1 && i != len(m.copyItems.items)-1 {
				// Last Entry we can render, but there are more that one left
				clipboardRender += strconv.Itoa(len(m.copyItems.items)-i) + " item left...."
			} else {
				fileInfo, err := os.Stat(m.copyItems.items[i])
				if err != nil {
					slog.Error("Clipboard render function get item state ", "error", err)
				}
				if !os.IsNotExist(err) {
					clipboardRender += clipboardPrettierName(m.copyItems.items[i], footerWidth(m.fullWidth)-3, fileInfo.IsDir(), false)
				}
			}
		}
	}
	bottomWidth := 0

	if m.fullWidth%3 != 0 {
		bottomWidth = footerWidth(m.fullWidth + m.fullWidth%3 + 2)
	} else {
		bottomWidth = footerWidth(m.fullWidth)
	}
	clipboardRender = clipboardBorder(m.footerHeight, bottomWidth, Config.BorderBottom).Render(clipboardRender)

	return clipboardRender
}

func (m *model) terminalSizeWarnRender() string {
	fullWidthString := strconv.Itoa(m.fullWidth)
	fullHeightString := strconv.Itoa(m.fullHeight)
	minimumWidthString := strconv.Itoa(minimumWidth)
	minimumHeightString := strconv.Itoa(minimumHeight)
	if m.fullHeight < minimumHeight {
		fullHeightString = terminalTooSmall.Render(fullHeightString)
	}
	if m.fullWidth < minimumWidth {
		fullWidthString = terminalTooSmall.Render(fullWidthString)
	}
	fullHeightString = terminalCorrectSize.Render(fullHeightString)
	fullWidthString = terminalCorrectSize.Render(fullWidthString)

	heightString := mainStyle.Render(" Height = ")
	return fullScreenStyle(m.fullHeight, m.fullWidth).Render(`Terminal size too small:` + "\n" +
		"Width = " + fullWidthString +
		heightString + fullHeightString + "\n\n" +

		"Needed for current config:" + "\n" +
		"Width = " + terminalCorrectSize.Render(minimumWidthString) +
		heightString + terminalCorrectSize.Render(minimumHeightString))
}

func (m *model) terminalSizeWarnAfterFirstRender() string {
	minimumWidthInt := Config.SidebarWidth + 20*len(m.fileModel.filePanels) + 20 - 1
	minimumWidthString := strconv.Itoa(minimumWidthInt)
	fullWidthString := strconv.Itoa(m.fullWidth)
	fullHeightString := strconv.Itoa(m.fullHeight)
	minimumHeightString := strconv.Itoa(minimumHeight)

	if m.fullHeight < minimumHeight {
		fullHeightString = terminalTooSmall.Render(fullHeightString)
	}
	if m.fullWidth < minimumWidthInt {
		fullWidthString = terminalTooSmall.Render(fullWidthString)
	}
	fullHeightString = terminalCorrectSize.Render(fullHeightString)
	fullWidthString = terminalCorrectSize.Render(fullWidthString)

	heightString := mainStyle.Render(" Height = ")
	return fullScreenStyle(m.fullHeight, m.fullWidth).Render(`You change your terminal size too small:` + "\n" +
		"Width = " + fullWidthString +
		heightString + fullHeightString + "\n\n" +

		"Needed for current config:" + "\n" +
		"Width = " + terminalCorrectSize.Render(minimumWidthString) +
		heightString + terminalCorrectSize.Render(minimumHeightString))
}

func (m *model) typineModalRender() string {
	previewPath := filepath.Join(m.typingModal.location, m.typingModal.textInput.Value())

	fileLocation := filePanelTopDirectoryIconStyle.Render(" "+icon.Directory+icon.Space) +
		filePanelTopPathStyle.Render(truncateTextBeginning(previewPath, modalWidth-4, "...")) + "\n"

	confirm := modalConfirm.Render(" (" + hotkeys.ConfirmTyping[0] + ") Create ")
	cancel := modalCancel.Render(" (" + hotkeys.CancelTyping[0] + ") Cancel ")

	tip := confirm +
		lipgloss.NewStyle().Background(modalBGColor).Render("           ") +
		cancel

	return modalBorderStyle(modalHeight, modalWidth).Render(fileLocation + "\n" + m.typingModal.textInput.View() + "\n\n" + tip)
}

func (m *model) introduceModalRender() string {
	title := sidebarTitleStyle.Render(" Thanks for using superfile!!") + modalStyle.Render("\n You can read the following information before starting to use it!")
	vimUserWarn := processErrorStyle.Render("  ** Very importantly ** If you are a Vim/Nvim user, go to:\n  https://superfile.netlify.app/configure/custom-hotkeys/ to change your hotkey settings!")
	subOne := sidebarTitleStyle.Render("  (1)") + modalStyle.Render(" If this is your first time, make sure you read:\n      https://superfile.netlify.app/getting-started/tutorial/")
	subTwo := sidebarTitleStyle.Render("  (2)") + modalStyle.Render(" If you forget the relevant keys during use,\n      you can press \"?\" (shift+/) at any time to query the keys!")
	subThree := sidebarTitleStyle.Render("  (3)") + modalStyle.Render(" For more customization you can refer to:\n      https://superfile.netlify.app/")
	subFour := sidebarTitleStyle.Render("  (4)") + modalStyle.Render(" Thank you again for using superfile.\n      If you have any questions, please feel free to ask at:\n      https://github.com/yorukot/superfile\n      Of course, you can always open a new issue to share your idea \n      or report a bug!")
	return firstUseModal(m.helpMenu.height, m.helpMenu.width).Render(title + "\n\n" + vimUserWarn + "\n\n" + subOne + "\n\n" + subTwo + "\n\n" + subThree + "\n\n" + subFour + "\n\n")
}

func (m *model) warnModalRender() string {
	title := m.warnModal.title
	content := m.warnModal.content
	confirm := modalConfirm.Render(" (" + hotkeys.Confirm[0] + ") Confirm ")
	cancel := modalCancel.Render(" (" + hotkeys.Quit[0] + ") Cancel ")
	tip := confirm + lipgloss.NewStyle().Background(modalBGColor).Render("           ") + cancel
	return modalBorderStyle(modalHeight, modalWidth).Render(title + "\n\n" + content + "\n\n" + tip)
}

func (m *model) helpMenuRender() string {
	helpMenuContent := ""
	maxKeyLength := 0

	for _, data := range m.helpMenu.data {
		totalKeyLen := 0
		for _, key := range data.hotkey {
			totalKeyLen += len(key)
		}
		saprateLen := len(data.hotkey) - 1*3
		if data.subTitle == "" && totalKeyLen+saprateLen > maxKeyLength {
			maxKeyLength = totalKeyLen + saprateLen
		}
	}

	valueLength := m.helpMenu.width - maxKeyLength - 2
	if valueLength < m.helpMenu.width/2 {
		valueLength = m.helpMenu.width/2 - 2
	}

	renderHotkeyLength := 0
	totalTitleCount := 0
	cursorBeenTitleCount := 0

	for i, data := range m.helpMenu.data {
		if data.subTitle != "" {
			if i < m.helpMenu.cursor {
				cursorBeenTitleCount++
			}
			totalTitleCount++
		}
	}

	for i := m.helpMenu.renderIndex; i < m.helpMenu.height+m.helpMenu.renderIndex && i < len(m.helpMenu.data); i++ {
		hotkey := ""

		if m.helpMenu.data[i].subTitle != "" {
			continue
		}

		for i, key := range m.helpMenu.data[i].hotkey {
			if i != 0 {
				hotkey += " | "
			}
			hotkey += key
		}

		if len(helpMenuHotkeyStyle.Render(hotkey)) > renderHotkeyLength {
			renderHotkeyLength = len(helpMenuHotkeyStyle.Render(hotkey))
		}
	}

	for i := m.helpMenu.renderIndex; i < m.helpMenu.height+m.helpMenu.renderIndex && i < len(m.helpMenu.data); i++ {

		if i != m.helpMenu.renderIndex {
			helpMenuContent += "\n"
		}

		if m.helpMenu.data[i].subTitle != "" {
			helpMenuContent += helpMenuTitleStyle.Render(" " + m.helpMenu.data[i].subTitle)
			continue
		}

		hotkey := ""
		description := truncateText(m.helpMenu.data[i].description, valueLength, "...")

		for i, key := range m.helpMenu.data[i].hotkey {
			if i != 0 {
				hotkey += " | "
			}
			hotkey += key
		}

		cursor := "  "
		if m.helpMenu.cursor == i {
			cursor = filePanelCursorStyle.Render(icon.Cursor + " ")
		}
		helpMenuContent += cursor + modalStyle.Render(fmt.Sprintf("%*s%s", renderHotkeyLength, helpMenuHotkeyStyle.Render(hotkey+" "), modalStyle.Render(description)))
	}

	bottomBorder := generateFooterBorder(fmt.Sprintf("%s/%s", strconv.Itoa(m.helpMenu.cursor+1-cursorBeenTitleCount), strconv.Itoa(len(m.helpMenu.data)-totalTitleCount)), m.helpMenu.width-2)

	return helpMenuModalBorderStyle(m.helpMenu.height, m.helpMenu.width, bottomBorder).Render(helpMenuContent)
}

func (m *model) sortOptionsRender() string {
	panel := m.fileModel.filePanels[m.filePanelFocusIndex]
	sortOptionsContent := modalTitleStyle.Render(" Sort Options") + "\n\n"
	for i, option := range panel.sortOptions.data.options {
		cursor := " "
		if i == panel.sortOptions.cursor {
			cursor = filePanelCursorStyle.Render(icon.Cursor)
		}
		sortOptionsContent += cursor + modalStyle.Render(" "+option) + "\n"
	}
	bottomBorder := generateFooterBorder(fmt.Sprintf("%s/%s", strconv.Itoa(panel.sortOptions.cursor+1), strconv.Itoa(len(panel.sortOptions.data.options))), panel.sortOptions.width-2)

	return sortOptionsModalBorderStyle(panel.sortOptions.height, panel.sortOptions.width, bottomBorder).Render(sortOptionsContent)
}

func readFileContent(filepath string, maxLineLength int, previewLine int) (string, error) {
	// String builder is much better for efficiency
	// See - https://stackoverflow.com/questions/1760757/how-to-efficiently-concatenate-strings-in-go/47798475#47798475
	var resultBuilder strings.Builder
	file, err := os.Open(filepath)
	if err != nil {
		return resultBuilder.String(), err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineCount := 0
	for scanner.Scan() {
		line := scanner.Text()
		if len(line) > maxLineLength {
			line = line[:maxLineLength]
		}
		// This is critical to avoid layout break, removes non Printable ASCII control characters.
		line = makePrintable(line)
		resultBuilder.WriteString(line + "\n")
		lineCount++
		if previewLine > 0 && lineCount >= previewLine {
			break
		}
	}
	// returns the first non-EOF error that was encountered by the [Scanner]
	return resultBuilder.String(), scanner.Err()
}

func (m *model) filePreviewPanelRender() string {
	previewLine := m.mainPanelHeight + 2
	m.fileModel.filePreview.width += m.fullWidth - Config.SidebarWidth - m.fileModel.filePreview.width - ((m.fileModel.width + 2) * len(m.fileModel.filePanels)) - 2

	panel := m.fileModel.filePanels[m.filePanelFocusIndex]
	box := filePreviewBox(previewLine, m.fileModel.filePreview.width)

	if len(panel.element) == 0 {
		return box.Render("\n --- " + icon.Error + " No content to preview ---")
	}

	itemPath := panel.element[panel.cursor].location

	fileInfo, err := os.Stat(itemPath)

	if err != nil {
		slog.Error("Error get file info", "error", err)
		return box.Render("\n --- " + icon.Error + " Error get file info ---")
	}

	ext := filepath.Ext(itemPath)
	// check if the file is unsupported file, cuz pdf will cause error
	if ext == ".pdf" || ext == ".torrent" {
		return box.Render("\n --- " + icon.Error + " Unsupported formats ---")
	}

	if fileInfo.IsDir() {
		directoryContent := ""
		dirPath := itemPath

		files, err := os.ReadDir(dirPath)
		if err != nil {
			slog.Error("Error render directory preview", "error", err)
			return box.Render("\n --- " + icon.Error + " Error render directory preview ---")
		}

		if len(files) == 0 {
			return box.Render("\n --- empty ---")
		}

		sort.Slice(files, func(i, j int) bool {
			if files[i].IsDir() && !files[j].IsDir() {
				return true
			}
			if !files[i].IsDir() && files[j].IsDir() {
				return false
			}
			return files[i].Name() < files[j].Name()
		})

		for i := 0; i < previewLine && i < len(files); i++ {
			file := files[i]
			directoryContent += prettierDirectoryPreviewName(file.Name(), file.IsDir(), filePanelBGColor)
			if i != previewLine-1 && i != len(files)-1 {
				directoryContent += "\n"
			}
		}
		directoryContent = checkAndTruncateLineLengths(directoryContent, m.fileModel.filePreview.width)
		return box.Render(directoryContent)
	}

	if isImageFile(itemPath) {
		if !m.fileModel.filePreview.open {
			// Todo : These variables can be pre rendered for efficiency and less duplicacy
			return box.Render("\n --- Preview panel is closed ---")
		}

		if !Config.ShowImagePreview {
			return box.Render("\n --- Image preview is disabled ---")
		}

		ansiRender, err := filepreview.ImagePreview(itemPath, m.fileModel.filePreview.width, previewLine, theme.FilePanelBG)
		if err == image.ErrFormat {
			return box.Render("\n --- " + icon.Error + " Unsupported image formats ---")
		}

		if err != nil {
			slog.Error("Error covernt image to ansi", "error", err)
			return box.Render("\n --- " + icon.Error + " Error covernt image to ansi ---")
		}

		return box.AlignVertical(lipgloss.Center).AlignHorizontal(lipgloss.Center).Render(ansiRender)
	}

	format := lexers.Match(filepath.Base(itemPath))

	if format == nil {
		isText, err := isTextFile(itemPath)
		if err != nil {
			slog.Error("Error while checking text file", "error", err)
			return box.Render("\n --- " + icon.Error + " Error get file info ---")
		} else if !isText {
			return box.Render("\n --- " + icon.Error + " Unsupported formats ---")
		}
	}

	// At this point either format is not nil, or we can read the file
	fileContent, err := readFileContent(itemPath, m.fileModel.width+20, previewLine)
	if err != nil {
		slog.Error("Error open file", "error", err)
		return box.Render("\n --- " + icon.Error + " Error open file ---")
	}

	if fileContent == "" {
		return box.Render("\n --- empty ---")
	}

	// We know the format of file, and we can apply syntax highlighting
	if format != nil {
		background := ""
		if !Config.TransparentBackground {
			background = theme.FilePanelBG
		}
		if Config.CodePreviewer == "bat" {
			if batCmd == "" {
				return box.Render("\n --- " + icon.Error + " 'bat' is not installed or not found. ---\n --- Cannot render file preview. ---")
			}
			fileContent, err = getBatSyntaxHighlightedContent(itemPath, previewLine, background)
		} else {
			fileContent, err = ansichroma.HightlightString(fileContent, format.Config().Name, theme.CodeSyntaxHighlightTheme, background)
		}
		if err != nil {
			slog.Error("Error render code highlight", "error", err)
			return box.Render("\n --- " + icon.Error + " Error render code highlight ---")
		}
	}

	fileContent = checkAndTruncateLineLengths(fileContent, m.fileModel.filePreview.width)
	return box.Render(fileContent)
}

func (m *model) commandLineInputBoxRender() string {
	return m.commandLine.input.View()
}

func getBatSyntaxHighlightedContent(itemPath string, previewLine int, background string) (string, error) {
	fileContent := ""
	// --plain: use the plain style without line numbers and decorations
	// --force-colorization: force colorization for non-interactive shell output
	// --line-range <:m>: only read from line 1 to line "m"
	batArgs := []string{itemPath, "--plain", "--force-colorization", "--line-range", fmt.Sprintf(":%d", previewLine-1)}

	// set timeout for the external command execution to 500ms max
	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(ctx, batCmd, batArgs...)

	fileContentBytes, err := cmd.Output()
	if err != nil {
		slog.Error("Error render code highlight", "error", err)
		return "", err
	}

	fileContent = string(fileContentBytes)
	if !Config.TransparentBackground {
		fileContent = setBatBackground(fileContent, background)
	}
	return fileContent, nil
}

func setBatBackground(input string, background string) string {
	tokens := strings.Split(input, "\x1b[0m")
	backgroundStyle := lipgloss.NewStyle().Background(lipgloss.Color(background))
	for idx, token := range tokens {
		tokens[idx] = backgroundStyle.Render(token)
	}
	return strings.Join(tokens, "\x1b[0m")
}
