package main

import (
	"bytes"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"syscall"
)

type Builder struct {
	Ext string `json:"ext"`
	Bin string `json:"bin"`
}

type Site struct {
	Name    string   `json:"name"`
	SrcRoot string   `json:"srcRoot"`
	DstRoot string   `json:"dstRoot"`
	TplPath string   `json:"tplPath"`
	Env     []string `json:"env,omitempty"`
}

type Config struct {
	Sites   []*Site  `json:"sites"`
	Builder Builder  `json:"builder"`
	RunCmd  []string `json:"runCmd"`
}

var (
	ConfigPath = flag.String("c", "config.json", "Configuration file")
	WorkingDir = flag.String("w", ".", "Working directory")
)

func main() {
	flag.Parse()
	os.Chdir(*WorkingDir)
	config, err := readConfig(*ConfigPath)
	if err != nil {
		log.Printf("cannot read config: %v", err)
	}
	for _, site := range config.Sites {
		if err := config.build(site); err != nil {
			log.Fatalf("could not build site %s: %v", site.SrcRoot, err)
		}
	}
}

func readConfig(configPath string) (*Config, error) {
	b, err := os.ReadFile(configPath)
	if err != nil {
		return nil, err
	}
	config := new(Config)
	if err := json.Unmarshal(b, config); err != nil {
		return nil, err
	}
	return config, nil
}

func (config *Config) build(site *Site) error {
	config.clean(site)
	return filepath.WalkDir(site.SrcRoot, func(path string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == site.SrcRoot {
			return nil
		}
		srcInfo, err := ent.Info()
		if err != nil {
			return err
		}
		if srcInfo.IsDir() {
			// If the file is a directory, we simply create a directory with the
			// same name under the corresponding directory in the dst tree.
			eqPath := filepath.Join(site.DstRoot, strings.TrimPrefix(path, site.SrcRoot))
			if err := os.MkdirAll(eqPath, 0755); err != nil {
				return err
			}
		} else {
			// If the file is a file to be built, we build it and write
			// the result in the dst tree as html file. If the file is of another
			// type we create a hard link to this file under the corresponding
			// directory in the dst tree.
			srcInfo, err := os.Stat(path)
			if err != nil {
				return err
			}
			eqPath := filepath.Join(site.DstRoot, strings.TrimPrefix(path, site.SrcRoot))
			ext := filepath.Ext(eqPath)
			if ext == config.Builder.Ext {
				eqPath = strings.TrimSuffix(eqPath, ext)
				eqPath += ".html"
				dstInfo, err := os.Stat(eqPath)
				if err != nil && errors.Is(err, os.ErrNotExist) {
					if err := config.buildPage(site, path, eqPath); err != nil {
						return err
					}
					fmt.Printf(" + %s\n", eqPath)
				} else if err == nil && srcInfo.ModTime().After(dstInfo.ModTime()) {
					// Rebuild the page if it has been updated in the src file tree.
					if err := config.buildPage(site, path, eqPath); err != nil {
						return err
					}
					fmt.Printf(" ^ %s\n", eqPath)
				}
			} else {
				dstInfo, err := os.Stat(eqPath)
				if err != nil && errors.Is(err, os.ErrNotExist) {
					if err := os.Link(path, eqPath); err != nil {
						return err
					}
					fmt.Printf(" + %s\n", eqPath)
				} else if err == nil && srcInfo.ModTime().After(dstInfo.ModTime()) {
					// Update the link if the resource in the src file tree has been updated.
					if err := os.Remove(eqPath); err != nil {
						return err
					}
					if err := os.Link(path, eqPath); err != nil {
						return err
					}
					fmt.Printf(" ^ %s\n", eqPath)
				}
			}
		}
		return nil
	})
}

func (config *Config) buildPage(site *Site, srcPath, dstPath string) error {
	b, err := os.ReadFile(site.TplPath)
	if err != nil {
		return err
	}
	templateString := string(b)
	re := regexp.MustCompile(`(?ms)^\s*%{(.*?)(?ms)^}%`)
	built := re.ReplaceAllStringFunc(templateString, func(match string) string {
		submatches := re.FindStringSubmatch(match)
		if len(submatches) < 2 {
			return match
		}
		cmdStr := submatches[1]
		cmdArgs := append(config.RunCmd, cmdStr)
		fmt.Println(cmdArgs)
		cmd := exec.Command(cmdArgs[0], cmdArgs[len(config.RunCmd)-1:]...)
		cmd.Env = os.Environ()
		srcBase := filepath.Base(srcPath)

		// Configure template environment variables (including variables added in config).
		cmd.Env = append(cmd.Env,
			"page_name="+strings.TrimSuffix(srcBase, filepath.Ext(srcBase)),
			"builder="+config.Builder.Bin,
			"site_name="+site.Name,
			"src_path="+srcPath,
			"dst_path="+dstPath,
		)
		cmd.Env = append(cmd.Env, site.Env...)

		var stdout bytes.Buffer
		cmd.Stdout = &stdout
		if err := cmd.Run(); err != nil {
			return fmt.Sprintf("%v", err)
		}
		return stdout.String()
	})
	os.WriteFile(dstPath, []byte(built), 0644)
	return nil
}

func (config *Config) clean(site *Site) error {
	return filepath.WalkDir(site.DstRoot, func(path string, ent fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == site.DstRoot {
			return nil
		}
		dstInfo, err := ent.Info()
		if err != nil {
			return err
		}
		if dstInfo.IsDir() {
			// If the file is a directory, we check that a file with the same name and
			// that is a directory too exists in the src tree, if not we delete it from
			// the dst tree.
			eqPath := filepath.Join(site.SrcRoot, strings.TrimPrefix(path, site.DstRoot))
			srcInfo, err := os.Stat(eqPath)
			if err != nil && errors.Is(err, os.ErrNotExist) || !srcInfo.IsDir() {
				os.RemoveAll(path)
				fmt.Printf(" - %s/*\n", path)
			}
		} else {
			// If the file is not a directory, we simply check that a file
			// with the same name and the same inode exists in the src tree, if not we
			// delete it from the dst tree.
			ext := filepath.Ext(path)
			eqPath := filepath.Join(site.SrcRoot, strings.TrimPrefix(path, site.DstRoot))
			if ext == ".html" {
				eqPath = strings.TrimSuffix(eqPath, ".html")
				eqPath += config.Builder.Ext
			}
			srcInfo, err := os.Stat(eqPath)
			var (
				srcStat *syscall.Stat_t
				dstStat *syscall.Stat_t
				ok      bool
			)
			if err == nil {
				srcStat, ok = srcInfo.Sys().(*syscall.Stat_t)
				if !ok {
					return fmt.Errorf("not a syscall: syscall.Stat_t")
				}
				dstStat, ok = dstInfo.Sys().(*syscall.Stat_t)
				if !ok {
					return fmt.Errorf("not a syscall: syscall.Stat_t")
				}
			}
			if err != nil && errors.Is(err, os.ErrNotExist) || (ext != ".html" && srcStat.Ino != dstStat.Ino) {
				os.RemoveAll(path)
				fmt.Printf(" - %s\n", path)
			}
		}
		return nil
	})
}
