package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"text/template"

	"github.com/myitcv/govim"
	"github.com/myitcv/govim/testsetup"
	"github.com/rogpeppe/go-internal/semver"
)

var thematrix []build

type build struct {
	goversion  string
	vimversion string
	vimflavor  govim.Flavor
	vimcommand string
	env        map[string]string
}

func (b build) dup() build {
	res := b
	res.env = make(map[string]string)
	for k, v := range b.env {
		res.env[k] = v
	}
	return res
}

type matrixstep func(build) []build

func buildmatrix() []build {
	if thematrix != nil {
		return thematrix
	}
	for _, v := range testsetup.VimVersions {
		thematrix = append(thematrix, build{
			vimversion: v.Version(),
			vimflavor:  v.Flavor(),
			vimcommand: v.Command(),
		})
	}
	steps := []matrixstep{
		expGoVersions,
	}
	for _, step := range steps {
		for i := 0; i < len(thematrix); {
			var newmat []build
			newmat = append(newmat, thematrix[:i]...)
			b := thematrix[i]
			var post []build
			if i < len(thematrix)-1 {
				post = thematrix[i+1:]
			}
			nb := step(b)
			i += len(nb)
			newmat = append(newmat, nb...)
			newmat = append(newmat, post...)
			thematrix = newmat
		}
	}
	return thematrix
}

func expGoVersions(b build) (res []build) {
	for _, v := range testsetup.GoVersions {
		gv := b.dup()
		gv.goversion = v
		res = append(res, gv)
	}
	return
}

// genconfig is a very basic templater that removes the need for hand-maintaining
// a couple of files. It is the source of the build matrix in the Travis config,
// and also the source of the default versions used in buildGovimImage.sh. Should
// be run from the root of the repo
func main() {
	writeMaxVersions()
	writeTravisYml()
}

func writeMaxVersions() {
	var vs struct {
		MaxGoVersion     string
		MaxVimVersion    string
		MaxGvimVersion   string
		MaxNeovimVersion string
		GoVersions       string
		VimVersions      string
		GvimVersions     string
		NeovimVersions   string
		VimCommand       string
		GvimCommand      string
		NeovimCommand    string
		ValidFlavors     string
	}
	vs.VimCommand = strconv.Quote(testsetup.VimCommand.String())
	vs.GvimCommand = strconv.Quote(testsetup.GvimCommand.String())
	vs.NeovimCommand = strconv.Quote(testsetup.NeovimCommand.String())
	vs.MaxGoVersion = testsetup.GoVersions[len(testsetup.GoVersions)-1]

	goVersionsSet := make(map[string]bool)
	vimVersionsSet := make(map[string]bool)
	gvimVersionsSet := make(map[string]bool)
	neovimVersionsSet := make(map[string]bool)

	for _, b := range buildmatrix() {
		goVersionsSet[b.goversion] = true
		switch b.vimflavor {
		case govim.FlavorVim:
			vimVersionsSet[b.vimversion] = true
		case govim.FlavorGvim:
			gvimVersionsSet[b.vimversion] = true
		case govim.FlavorNeovim:
			neovimVersionsSet[b.vimversion] = true
		default:
			panic(fmt.Errorf("don't know about flavor %v", b.vimflavor))
		}
	}

	goVersions := setToList(goVersionsSet)
	vimVersions := setToList(vimVersionsSet)
	gvimVersions := setToList(gvimVersionsSet)
	neovimVersions := setToList(neovimVersionsSet)

	sort.Slice(goVersions, func(i, j int) bool {
		lhs := strings.ReplaceAll(goVersions[i], "go", "v")
		rhs := strings.ReplaceAll(goVersions[j], "go", "v")
		return semver.Compare(lhs, rhs) < 0
	})
	sort.Slice(vimVersions, func(i, j int) bool {
		return semver.Compare(vimVersions[i], vimVersions[j]) < 0
	})
	sort.Slice(gvimVersions, func(i, j int) bool {
		return semver.Compare(gvimVersions[i], gvimVersions[j]) < 0
	})
	sort.Slice(neovimVersions, func(i, j int) bool {
		return semver.Compare(neovimVersions[i], neovimVersions[j]) < 0
	})

	if len(vimVersions) == 0 {
		panic(fmt.Errorf("found no vim versions"))
	}
	vs.MaxVimVersion = vimVersions[len(vimVersions)-1]
	vs.MaxGvimVersion = gvimVersions[len(gvimVersions)-1]
	vs.MaxNeovimVersion = neovimVersions[len(neovimVersions)-1]
	vs.GoVersions = strings.Join(goVersions, " ")
	vs.VimVersions = strings.Join(vimVersions, " ")
	vs.GvimVersions = strings.Join(gvimVersions, " ")
	vs.NeovimVersions = strings.Join(neovimVersions, " ")

	var flavStrings []string
	for _, f := range govim.Flavors {
		flavStrings = append(flavStrings, f.String())
	}
	vs.ValidFlavors = strings.Join(flavStrings, " ")
	writeFileFromTmpl(filepath.Join("_scripts", "gen_maxVersions_genconfig.bash"), maxVersions, vs)
}

// writeTravisYml assumes and writes a simple MxN matrix of Go versions and Vim versions
func writeTravisYml() {
	var entries []string
	for _, b := range buildmatrix() {
		// TODO when we add Neovim to the actually build, drop this skip
		if b.vimflavor == govim.FlavorNeovim {
			continue
		}
		var env []string
		for k, v := range b.env {
			env = append(env, fmt.Sprintf("%v=%q", k, v))
		}
		var space string
		if len(env) > 0 {
			space = " "
		}
		entries = append(entries, fmt.Sprintf("    - GO_VERSION=%q VIM_FLAVOR=%q VIM_VERSION=%q VIM_COMMAND=%q%v%v", b.goversion, b.vimflavor, b.vimversion, b.vimcommand, space, strings.Join(env, " ")))
	}
	writeFileFromTmpl(".travis.yml", travisYml, entries)
}

func writeFileFromTmpl(path string, tmpl string, v interface{}) {
	fi, err := os.Create(path)
	if err != nil {
		panic(err)
	}
	t := template.Must(template.New(path).Parse(tmpl))
	if err := t.Execute(fi, v); err != nil {
		panic(err)
	}
	if err := fi.Close(); err != nil {
		panic(err)
	}
}

const travisYml = `# Code generated by genconfig. DO NOT EDIT.
language: generic
sudo: required

notifications:
  email:
    recipients:
      - secure: RUg36ZsUsNIjUokaeLH/XykeRncSb7Fdwh2RnHq8+YSWQS2FgnhtBBiKEr7+/xMW3Zw0slpCdWt1CcoMF7rsAUwgQK4rNkGadHlRYqvBjB+45paBp9duEelEOP2VTjdEPRPWyg7e8uQYqvPHEIAmz99QGhWxbVSoeJaThRycOYxs3J+Y4mVwbrEMH5H9jTwMdojKawvBKTKrodqowPPtIpFCLv4L/KqCnvhfbfFbAiM76Fb22MwEr9YYPW6oJF/rbta9KjYCYG5SiF0qXcIvXTbB2e0EEfsArklQepDaErHR28yjDMLkqHHviHtQPq4vIatWvodVfvnDIOBQEo1EF+4kHwfQm+VYsm6SLyO47sreG7j9/oBHo8kPF0KM085oFzKRRC+yGrCvG6IunfFZ2g+9On283pKD0/SIjxKCkOlLxadXI5GQKvWFGBQXACga4PdOASLY8k9Tj0HtFmCb/eRd/cHvma1/0E/LRkwDpuAmUdO0e6567O9pYuxkQYqgbzHuS9av37aw8KiunnWqCIg1NgSOoMjhsw12NDRIbHtdKvkP5Ic0HFig5HK3kRc6t7yD8aHSTqbqEMnQ2icDpTVnbZLZksp/TaUBmQRwUVdPpdS1isB+hd/vj4YQEyM/v9XKzOT50gKwUVWNDAyWs1iyAae40kS6oSvYrYSFhvM=
    on_success: always
    on_failure: always

branches:
  only:
    - master

os:
  - linux

env:
  global:
    - GO111MODULE=on
    - GOPROXY=https://proxy.golang.org
  matrix:
{{range . -}}{{.}}
{{end}}
# Add this before_install until we have a definitive resolution on
# https://travis-ci.community/t/files-in-checkout-have-eol-changed-from-lf-to-crlf/349/2
before_install:
  - cd ../..
  - mv $TRAVIS_REPO_SLUG _old
  - git config --global core.autocrlf false
  - git clone --depth=50 _old $TRAVIS_REPO_SLUG
  - cd $TRAVIS_REPO_SLUG

before_install:
  - docker --version
  - ./_scripts/buildGovimImage.sh

script:
  - ./_scripts/runDockerRun.sh
`

const maxVersions = `# Code generated by genconfig. DO NOT EDIT.
export GO_VERSIONS="{{.GoVersions}}"
export VIM_VERSIONS="{{.VimVersions}}"
export GVIM_VERSIONS="{{.GvimVersions}}"
export NEOVIM_VERSIONS="{{.NeovimVersions}}"

export MAX_GO_VERSION={{.MaxGoVersion}}
export MAX_VIM_VERSION={{.MaxVimVersion}}
export MAX_GVIM_VERSION={{.MaxGvimVersion}}
export MAX_NEOVIM_VERSION={{.MaxNeovimVersion}}

export DEFAULT_VIM_COMMAND={{.VimCommand}}
export DEFAULT_GVIM_COMMAND={{.GvimCommand}}
export DEFAULT_NEOVIM_COMMAND={{.NeovimCommand}}

export VALID_FLAVORS="{{.ValidFlavors}}"
`

func setToList(m map[string]bool) []string {
	var res []string
	for k := range m {
		res = append(res, k)
	}
	return res
}
