package profiling

import "github.com/spf13/pflag"

var (
	// Addr the address for starting profiling server
	Addr = ""
)

// AddFlags .
func AddFlags(fs *pflag.FlagSet) {
	fs.StringVarP(&Addr, "profiling-addr", "", Addr, "if not empty, start the profiling server at the given address")
}
