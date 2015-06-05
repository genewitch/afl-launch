package main

import (
	"crypto/rand"
	"flag"
	"io"
	"log"
	"os/exec"
	"path"
	"strconv"
	"strings"
)

const MAXFUZZERS = 256
const AFLNAME = "afl-fuzz"

var (
	flagNoMaster = flag.Bool("no-master", false, "Launch all instances with -S")
	flagNum      = flag.Int("n", 1, "Number of instances to launch")
	flagName     = flag.String("name", "", "Base name for instances. Names will be <output>/<BASE>-[M|S]<N>")
	flagTimeout  = flag.Int("t", -1, "afl-fuzz -t option (timeout)")
	flagMem      = flag.Int("m", -1, "afl-fuzz -m option (memory limit)")
	flagInput    = flag.String("i", "", "afl-fuzz -i option (input location)")
	flagExtras   = flag.String("x", "", "afl-fuzz -x option (extras location)")
	flagOutput   = flag.String("o", "", "afl-fuzz -o option (output location)")
	flagFile     = flag.String("f", "", "Filename template (substituted and passed via -f)")
)

func randomName(n int) (result string) {
	buf := make([]byte, n)
	_, err := io.ReadFull(rand.Reader, buf)
	if err != nil {
		panic(err)
	}
	for _, b := range buf {
		result += string(b%26 + 0x61)
	}
	return
}

func spawn(fuzzerName string, args []string) {

	// if the user wants to use a special location for the testfiles ( like a
	// ramdisk ) then they can provide any filename /path/to/whatever.xxx and
	// we'll sub out 'whatever' for the name of this fuzzer and keep the base
	// and the extension.
	if len(*flagFile) > 0 {
		base, _ := path.Split(*flagFile)
		ext := path.Ext(*flagFile)
		args = append(args, "-f", path.Join(base, fuzzerName+ext))
	}

	args = append(args, "--")
	args = append(args, flag.Args()...)
	cmd := exec.Command(AFLNAME, args...)
	err := cmd.Start()
	if err != nil {
		// If this fails to start it will be OS issues like no swap or rlimit
		// or something, so it's not something we can handle gracefully. It's
		// NOT the same as the afl-fuzz process exiting because the args are
		// incorrect.
		log.Fatalf(err.Error())
	}
	cmd.Process.Release()
	log.Printf("%s %s\n", AFLNAME, strings.Join(args, " "))
}

func main() {

	flag.Parse()
	if len(flag.Args()) < 2 {
		log.Fatalf("no command to fuzz, eg: targetname @@")
	}

	// can we find afl?
	_, err := exec.LookPath(AFLNAME)
	if err != nil {
		log.Fatalf("couldn't find %s in $PATH", AFLNAME)
	}
	// sanity for n
	if *flagNum > MAXFUZZERS {
		log.Fatalf("too many fuzzers: %d", *flagNum)
	}
	// sanity for name
	if len(*flagName) > 32 {
		log.Fatalf("base name too long (%d), must be <= 32", len(*flagName))
	}

	// collect the proxy args for afl-fuzz
	baseArgs := []string{}
	for _, v := range []string{"t", "m", "i", "x", "o"} {
		f := flag.Lookup(v)
		if f != nil && f.Value.String() != f.DefValue {
			baseArgs = append(baseArgs, "-"+v, f.Value.String())
		}
	}

	baseName := *flagName
	if len(baseName) == 0 {
		baseName = randomName(5)
	}

	// first instance is a master unless indicated otherwise
	if *flagNoMaster {
		name := baseName + "-" + "S" + "0"
		spawn(name, append(baseArgs, "-S", name))
	} else {
		name := baseName + "-" + "M" + "0"
		spawn(name, append(baseArgs, "-M", name))
	}

	// launch the rest
	for i := 1; i < *flagNum; i++ {
		name := baseName + "-" + "S" + strconv.Itoa(i)
		spawn(name, append(baseArgs, "-S", name))
	}
}