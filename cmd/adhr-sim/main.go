package main

import (
	"fmt"
	"os"
)

const version = "0.0.0-dev"

func main() {
	if err := run(os.Args[1:]); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run(args []string) error {
	if len(args) == 0 {
		printUsage(os.Stdout)
		return nil
	}
	switch args[0] {
	case "-h", "--help":
		printUsage(os.Stdout)
		return nil
	case "version":
		fmt.Println(version)
		return nil
	case "run":
		return runScenarioCommand(args[1:])
	case "sweep":
		return runSweepCommand(args[1:])
	case "regression-pack":
		return runRegressionPackCommand(args[1:])
	case "calibrate-tyt":
		return runCalibrateTYTCommand(args[1:])
	case "calibrate-channel":
		return runCalibrateChannelCommand(args[1:])
	case "score-audio":
		return runScoreAudioCommand(args[1:])
	case "convert-bench-log":
		return runConvertBenchLogCommand(args[1:])
	case "ptt-contention":
		return runPTTContentionCommand(args[1:])
	case "hardware-readiness":
		return runHardwareReadinessCommand(args[1:])
	case "readiness-dashboard":
		return runReadinessDashboardCommand(args[1:])
	case "peer-acceptance":
		return runPeerAcceptanceCommand(args[1:])
	case "control-comparison":
		return runControlComparisonCommand(args[1:])
	case "opus-budget":
		return runOpusBudgetCommand(args[1:])
	case "codec-budget":
		return runCodecBudgetCommand(args[1:])
	default:
		return fmt.Errorf("unknown command %q; see --help", args[0])
	}
}

func printUsage(w *os.File) {
	fmt.Fprintln(w, "adhr-sim — Ad Hoc Radio simulator CLI")
	fmt.Fprintln(w, "version:", version)
	fmt.Fprintln(w, "usage: adhr-sim <command>")
	fmt.Fprintln(w, "commands:")
	fmt.Fprintln(w, "  run <scenario.json> --metrics <path>")
	fmt.Fprintln(w, "  sweep --out <dir>")
	fmt.Fprintln(w, "  regression-pack <pack.json> --out <dir>")
	fmt.Fprintln(w, "  calibrate-tyt <measurement.json> --report <path>")
	fmt.Fprintln(w, "  calibrate-channel <channel.json> --report <path>")
	fmt.Fprintln(w, "  score-audio <audio.json> --report <path>")
	fmt.Fprintln(w, "  convert-bench-log <raw-log.json> --out <canonical.json>")
	fmt.Fprintln(w, "  ptt-contention <scenario.json> --report <path>")
	fmt.Fprintln(w, "  hardware-readiness --channel <report.json> --turnaround <report.json> --audio <report.json> --report <path>")
	fmt.Fprintln(w, "  readiness-dashboard <regression-summary.json> --report <path>")
	fmt.Fprintln(w, "  peer-acceptance --report <path>")
	fmt.Fprintln(w, "  control-comparison --report <path>")
	fmt.Fprintln(w, "  opus-budget --input <trace.json> --report <path>")
	fmt.Fprintln(w, "  codec-budget --input <trace.json> --report <path>")
	fmt.Fprintln(w, "  version   print version and exit")
	fmt.Fprintln(w, "  --help    show this help")
}
