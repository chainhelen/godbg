package main

import (
	"fmt"
	"github.com/c-bata/go-prompt"
	"go.uber.org/zap"
	"os"
	"path"
	"strconv"
	"strings"
	"syscall"
)

func executor(input string) {
	logger.Debug("executor", zap.String("input", input))
	if len(input) == 0 {
		return
	}
	fs := input[0]

	switch fs {
	case 'q':
		if input == "q" || input == "quit"{
			if cmd.Process != nil {
				if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL); err != nil {
					// printErr(err)
				}
			}
			if os.Getenv("GODBG_TEST") != "" {
				return
			}
			os.Exit(0)
		}
	case 'b':
		sps := strings.Split(input, " ")
		if len(sps) == 2 && (sps[0] == "b" || sps[0] == "break") {
			filename, line, err := parseLoc(sps[1])
			if err != nil {
				printUnsupportCmd(input)
				return
			}
			if bInfo, err := bp.SetFileLineBreakPoint(filename, line); err != nil {
				if err == HasExistedBreakPointErr {
					printHasExistedBreakPoint(sps[1])
					return
				}
				if err == NotFoundSourceLineErr {
					printNotFoundSourceLineErr(sps[1])
					return
				}
				printErr(err)
				return
			} else {
				fmt.Fprintf(stdout,"godbg add %s:%d breakpoint successfully\n",bInfo.filename, bInfo.lineno)
			}
			return
		}
		if len(sps) == 2 && (sps[0] == "bc" || sps[0] == "bclear") {

			if sps[1] == "all" {
				tmp := make([]*BInfo, 0, len(bp.infos))
				for _, v := range bp.infos {
					if v.kind == USERBPTYPE {
						bp.disableBreakPoint(v)
					} else {
						tmp = append(tmp, v)
					}
				}
				bp.infos = tmp
				return
			}

			if needClearIndex, err := strconv.Atoi(sps[1]); err == nil {
				if needClearIndex > len(bp.infos) {
					printErr(fmt.Errorf("can't find breakpoint index %d", needClearIndex))
				}
				count := 0
				for i, v := range bp.infos {
					if v.kind == USERBPTYPE {
						count++
						if count == needClearIndex {
							bp.disableBreakPoint(v)
							bp.infos = append(bp.infos[:i], bp.infos[(i + 1):len(bp.infos)]...)
							fmt.Fprintf(stdout, "clear breakpoint %d successfully, resort breakpoint again\n", needClearIndex)
							return
						}
					}
				}
				printErr(fmt.Errorf("can't find breakpoint index %d", needClearIndex))
				return
			}
		}
		if len(sps) == 1 && (sps[0] == "bl") {
			count := 0
			for _, v := range bp.infos {
				if v.kind == USERBPTYPE {
					count++
					fmt.Fprintf(stdout,"%-2d. %s:%d, pc %d\n", count, v.filename, v.lineno, v.pc)
				}
			}
			if count == 0 {
				fmt.Fprintf(stdout,"there is no breakpoint\n")
			}
			return
		}
		if len(sps) == 2 && (sps[0] == "bl" && sps[1] == "all") {
			count := 0
			for _, v := range bp.infos {
				count++
				fmt.Fprintf(stdout,"%-2d. %s:%d, pc %d, type %s\n", count, v.filename, v.lineno, v.pc, v.kind.String())
			}
			if count == 0 {
				fmt.Fprintf(stdout,"there is no breakpoint\n")
			}
			return
		}
	case 'c':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "c" || sps[0] == "continue") {
			if cmd.Process == nil {
				printNoProcessErr()
				return
			}

			/*
			version 1
			if ok, err := bp.singleStepInstructionWithBreakpointCheck(); err != nil {
				printErr(err)
				return
			} else if ok {
				if err := bp.Continue(); err != nil {
					printErr(err)
					return
				}
				var s syscall.WaitStatus
				wpid, err := syscall.Wait4(cmd.Process.Pid, &s, syscall.WALL, nil)
				if err != nil {
					printErr(err)
					return
				}
				status := (syscall.WaitStatus)(s)
				if status.Exited() {
					// TODO
					if cmd.Process != nil && wpid == cmd.Process.Pid {
						printExit0(wpid)
					} else {
						printExit0(wpid)
					}
					cmd.Process = nil
					return
				}
			}*/

			/* version 2 */
			if err := bp.singleStepInstructionWithBreakpointCheck_v2(); err != nil {
				printErr(err)
				return
			}
			if err := bp.Continue(); err != nil {
				printErr(err)
				return
			}
			var (
				s syscall.WaitStatus
				pc uint64
			)
			wpid, err := syscall.Wait4(cmd.Process.Pid, &s, syscall.WALL, nil)
			if err != nil {
				printErr(err)
				return
			}

			if s.Exited() {
				printExit0(wpid)
				return
			}

			if s.StopSignal() != syscall.SIGTRAP {
				cmd.Process = nil
				fmt.Errorf("unknown waitstatus %v, signal %d", s, s.Signal())
				return
			}

			if pc, err = getPtracePc(); err != nil {
				printErr(err)
				return
			}
			fmt.Fprintf(stdout,"current process pc = %d\n", pc)
			if err = listFileLineByPtracePc(6); err != nil {
				printErr(err)
				return
			}
			return
		}
	case 's':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "s" || sps[0] == "step") {
			var (
				err error
				filename string
				lineno int
				oldfilename string
				oldlineno int
				pc uint64
				info *BInfo
				ok bool
			)
			if oldfilename, oldlineno, err = bi.getCurFileLineByPtracePc(); err != nil {
				printErr(err)
				return
			}
			for {
				if pc, err = getPtracePc(); err != nil {
					printErr(err)
					return
				}
				if filename, lineno, err = bi.pcTofileLine(pc); err != nil {
					printErr(err)
					return
				}

				if !(filename == oldfilename && lineno == oldlineno) {
					fmt.Fprintf(stdout,"current process pc = %d\n", pc)
					if err = listFileLineByPtracePc(6); err != nil {
						printErr(err)
						return
					}
					return
				}
				if info, ok = bp.findBreakPoint(pc - 1); ok {
					if err = bp.disableBreakPoint(info); err !=nil {
						printErr(err)
						return
					}
					defer bp.enableBreakPoint(info)
					if err = setPcRegister(pc - 1); err != nil {
						printErr(err)
						return
					}
				}
				if err = syscall.PtraceSingleStep(cmd.Process.Pid); err != nil {
					printErr(err)
					return
				}
				var s syscall.WaitStatus
				if _, err = syscall.Wait4(cmd.Process.Pid, &s, syscall.WALL, nil); err != nil {
					printErr(err)
					return
				}
				if s.Exited() {
					printExit0(cmd.Process.Pid)
					return
				}
				if s.StopSignal() != syscall.SIGTRAP {
					printErr(fmt.Errorf("unknown waitstatus %v, signal %d", s, s.Signal()))
					return
				}
			}
			return
		}
	case 'n':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "n" || sps[0] == "next") {
			var (
				err error
				//filename string
				//lineno int
				oldfilename string
				//oldlineno int
				pc uint64
				info *BInfo
				ok bool
				f *Function
			)
			if oldfilename, _, err = bi.getCurFileLineByPtracePc(); err != nil {
				printErr(err)
				return
			}

			if pc, err = getPtracePc(); err != nil {
				printErr(err)
				return
			}

			if f, err = findFunctionIncludePc(pc); err != nil {
				printErr(err)
				return
			}
			funclines := make(map[int]bool, f.highpc - f.lowpc)
			for v := f.lowpc; v < f.highpc;v++ {
				if _, line, err := bi.pcTofileLine(v); err != nil {
					printErr(err)
					return
				} else {
					funclines[line] = true
				}
			}
			for k, v := range funclines {
				if v {
					if curpc, err := bi.fileLineToPcForBreakPoint(oldfilename, k); err!=nil {
						printErr(err)
						return
					} else {
						if info, err = bp.SetInternalBreakPoint(curpc); err != nil && err != HasExistedBreakPointErr {
							printErr(err)
							return
						}
						if err == HasExistedBreakPointErr {
							err = nil
						} else {
							defer bp.disableBreakPoint(info)
							defer bp.clearInternalBreakPoint(curpc)
						}
					}
				}
			}

			for {
				if pc, err = getPtracePc(); err != nil {
					printErr(err)
					return
				}

				if info, ok = bp.findBreakPoint(pc - 1); ok {
					if err = bp.disableBreakPoint(info); err !=nil {
						printErr(err)
						return
					}
					defer bp.enableBreakPoint(info)
					if err = setPcRegister(pc - 1); err != nil {
						printErr(err)
						return
					}
					pc = pc - 1
				}
				if err = syscall.PtraceSingleStep(cmd.Process.Pid); err != nil {
					printErr(err)
					return
				}
				var s syscall.WaitStatus
				if _, err = syscall.Wait4(cmd.Process.Pid, &s, syscall.WALL, nil); err != nil {
					printErr(err)
					return
				}
				if s.Exited() {
					printExit0(cmd.Process.Pid)
					return
				}
				if s.StopSignal() != syscall.SIGTRAP {
					printErr(fmt.Errorf("unknown waitstatus %v, signal %d", s, s.Signal()))
					return
				}
				if pc, err = getPtracePc(); err != nil {
					printErr(err)
					return
				}
				if _, ok = bp.findBreakPoint(pc - 1); ok {
					if err := listFileLineByPtracePc(6); err != nil {
						printErr(err)
						return
					}
					return
				}
				if !(f.lowpc <= pc && pc < f.highpc) {
					if err := listFileLineByPtracePc(6); err != nil {
						printErr(err)
						return
					}
					return
				}
			}
			return
		}
	case 'l':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "l" || sps[0] == "list") {
			if err := listFileLineByPtracePc(6); err != nil {
				printErr(err)
				return
			}
			return
		}

		if len(sps) == 2 && (sps[0] == "l" || sps[0] == "list") {
			filename, line, err := parseLoc(sps[1])
			if err != nil {
				printUnsupportCmd(input)
				return
			}
			if err = listFileLine(filename, line, 6); err != nil {
				printErr(err)
				return
			}
			return
		}

		if len(sps) == 3 && (sps[0] == "l" || sps[0] == "list") {
			filename, line, err := parseLoc(sps[1])
			if err != nil {
				printUnsupportCmd(input)
				return
			}
			rangeline, err := strconv.Atoi(sps[2])
			if err != nil {
				printUnsupportCmd(input)
				return
			}
			if err = listFileLine(filename, line, rangeline); err != nil {
				printErr(err)
				return
			}
			return
		}
	case 'r':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "r" || sps[0] == "restart") {
			pid := 0
			if cmd.Process != nil {
				pid = cmd.Process.Pid
				if err := syscall.Kill(-cmd.Process.Pid, syscall.SIGSTOP); err != nil {
					printErr(err)
					return
				}
			}
			if pid != 0 {
				fmt.Fprintf(stdout, "  stop  old process pid %d\n", pid)
			}
			var err error
			if cmd, err = runexec(execfile); err != nil {
				printErr(err)
				logger.Error(err.Error(), zap.String("stage", "restart:runexec"), zap.String("execfile", execfile))
				return
			}
			if err = bp.SetBpWhenRestart(); err != nil {
				printErr(err)
				logger.Error(err.Error(), zap.String("stage", "restart:setbp"), zap.String("execfile", execfile))
				return
			}
			fmt.Fprintf(stdout, "restart new process pid %d \n", cmd.Process.Pid)
			return
		}
	case 'd':
		sps := strings.Split(input, " ")
		if len(sps) == 1 && (sps[0] == "disass" || sps[0] == "disassemble") {
			if err := listDisassembleByPtracePc(); err != nil {
				printErr(err)
				return
			}
			return
		}
	}
	printUnsupportCmd(input)
}

func complete(docs prompt.Document) []prompt.Suggest {
	sps := strings.Split(docs.Text, " ")

	s := make([]prompt.Suggest, 0)

	curWd, _ := os.Getwd()

	if len(sps) == 2 {
		if sps[0] == "b" || sps[0] == "break" || sps[0] == "l" || sps[0] == "list" {
			for filename := range bi.Sources {
				if strings.HasPrefix(filename, sps[1]) {
					if filename[0] == '/' {
						filename = filename[1:]
					}
					s = append(s, prompt.Suggest{Text: filename, Description:""})
				} else {

					inputPrefix := sps[1]
					if inputPrefixFilename := path.Join(curWd, inputPrefix); strings.HasPrefix(filename, inputPrefixFilename) {
						if len(inputPrefix) >= 2 && inputPrefix[:2] == "./" {
							inputPrefix = inputPrefix[2:]
						}
						needComplete := filename[len(inputPrefixFilename):]
						s = append(s, prompt.Suggest{Text: inputPrefix + needComplete, Description: ""})
					}
				}
				if len(s) >= 30 {
					return s
				}
			}
		}
	}
	return s
}
