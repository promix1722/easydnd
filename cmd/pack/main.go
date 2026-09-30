// Command pack validates, locks and exports installed JSON rule packs.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/promix1722/easydnd/internal/adapter/catalog/file"
)

func main() {
	in := flag.String("in", "data/srd_5.1", "comma-separated pack files/directories (dependencies included)")
	out := flag.String("out", "", "export the first pack to a new JSON file or directory")
	directory := flag.Bool("directory", false, "export directory representation")
	flag.Parse()
	if err := run(strings.Split(*in, ","), *out, *directory); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run(paths []string, out string, directory bool) error {
	registry, err := file.NewRegistry(paths, nil, "")
	if err != nil {
		return err
	}
	if out != "" {
		p, err := file.LoadPack(paths[0])
		if err != nil {
			return err
		}
		if directory {
			if err = file.SavePackDirectory(out, p); err != nil {
				return err
			}
		} else {
			b, err := file.EncodePack(p)
			if err != nil {
				return err
			}
			f, err := os.OpenFile(out, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0644)
			if err != nil {
				return err
			}
			_, err = f.Write(b)
			closeErr := f.Close()
			if err != nil {
				return err
			}
			if closeErr != nil {
				return closeErr
			}
		}
	}
	b, err := json.MarshalIndent(file.LockOf(registry.DefaultLock()), "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(b))
	return nil
}
