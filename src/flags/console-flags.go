package flags

import (
	"flag"
	"strconv"

	"go.uber.org/zap"
)

type stringFlag struct {
	set   bool
	value string
}

func (sf *stringFlag) Set(x string) error {
	sf.value = x
	sf.set = true
	return nil
}
func (sf *stringFlag) String() string {
	return sf.value
}
func (sf *stringFlag) Bool() bool {
	convert, _ := strconv.ParseBool(sf.value)
	return convert
}
func (sf *stringFlag) IsSet() bool {
	return sf.set
}

type ConsoleFlags struct {
	Mode      stringFlag
	NspFolder stringFlag
	Recursive stringFlag
	ExportCsv stringFlag
}

var mode stringFlag
var nspFolder stringFlag
var recursive stringFlag
var exportCsv stringFlag

func Initialize() {
	if flag.Parsed() {
		return
	}

	flag.Var(&mode, "m", "console or gui, overrides the gui flag in settings.json")
	flag.Var(&nspFolder, "f", "path to NSP folder")
	flag.Var(&recursive, "r", "recursively scan sub folders (true/false)")
	flag.Var(&exportCsv, "e", "if exists, output missing updates, dlcs and issues as csv")

	flag.Parse()
}

var (
	consoleFlagsInstance *ConsoleFlags
)

func GetValues() *ConsoleFlags {
	if consoleFlagsInstance != nil {
		return consoleFlagsInstance
	}
	if !flag.Parsed() {
		Initialize()
	}

	consoleFlagsInstance = &ConsoleFlags{
		Mode:      mode,
		NspFolder: nspFolder,
		Recursive: recursive,
		ExportCsv: exportCsv,
	}

	return consoleFlagsInstance
}

func LogFlags(sugar *zap.SugaredLogger) {
	logFlag(sugar, "m", mode)
	logFlag(sugar, "f", nspFolder)
	logFlag(sugar, "r", recursive)
	logFlag(sugar, "e", exportCsv)
}

func logFlag(sugar *zap.SugaredLogger, flagName string, flag stringFlag) {
	if !flag.set {
		sugar.Infof("[Flag -%v: %v]", flagName, "<missing>")
	} else {
		sugar.Infof("[Flag -%v: %v]", flagName, flag.value)
	}
}
