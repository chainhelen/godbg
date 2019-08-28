package main

import (
	"fmt"
	"github.com/c-bata/go-prompt"
	"go.uber.org/zap"
	"os"
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
					printErr(err)
				}
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
			}

			var (
				pc uint64
				err error
			)
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
	_ = docs
	return nil
}
