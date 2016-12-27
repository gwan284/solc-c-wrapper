package solc

/*
#cgo CPPFLAGS: -I${SRCDIR}/include/

#include <stdlib.h>
#include <solc.h>

static char** makeCharArray(int size)
{
	return calloc(sizeof(char*), size);
}

static void setArrayString(char **a, char *s, int n)
{
	a[n] = s;
}

static void freeCharArray(char **a, int size)
{
	int i;
	for (i = 0; i < size; i++)
	free(a[i]);
	free(a);
}

*/
import "C"

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"regexp"
	"strings"
)

var (
	versionRegexp = regexp.MustCompile("[0-9]+\\.[0-9]+\\.[0-9]+")
	solcParams = []string{
		"--combined-json", "bin,abi,userdoc,devdoc",
		"--add-std", // include standard lib contracts
		"--optimize", // code optimizer switched on
	}
)

type Contract struct {
	Code string       `json:"code"`
	Info ContractInfo `json:"info"`
}

type ContractInfo struct {
	Source          string      `json:"source"`
	Language        string      `json:"language"`
	LanguageVersion string      `json:"languageVersion"`
	CompilerVersion string      `json:"compilerVersion"`
	CompilerOptions string      `json:"compilerOptions"`
	AbiDefinition   interface{} `json:"abiDefinition"`
	UserDoc         interface{} `json:"userDoc"`
	DeveloperDoc    interface{} `json:"developerDoc"`
}

// Solidity contains information about the solidity compiler.
type Solidity struct {
	Version, FullVersion string
}

// --combined-output format
type solcOutput struct {
	Contracts map[string]struct{ Bin, Abi, Devdoc, Userdoc string }
	Version   string
}

// SolidityVersion runs solc and parses its version output.
func SolidityVersion() (*Solidity, error) {
	var stdout bytes.Buffer

	args := []string{"--version"}

	callCSolc(args)
	s := &Solidity{
		FullVersion: stdout.String(),
		Version:     versionRegexp.FindString(stdout.String()),
	}
	return s, nil
}

// CompileSolidity compiles all given Solidity source files.
func CompileSolidity(solc string, sourcefiles ...string) (map[string]*Contract, error) {
	if len(sourcefiles) == 0 {
		return nil, errors.New("solc: no source ")
	}
	source, err := slurpFiles(sourcefiles)
	if err != nil {
		return nil, err
	}

	var stderr, stdout bytes.Buffer
	args := append(solcParams, "--")
	//os.Stdout = &stdout
	//os.Stderr = &stderr

	ok := callCSolc(append(args, sourcefiles...))

	if !ok {
		return nil, fmt.Errorf("solc: %v\n%s", err, stderr.Bytes())
	}

	var output solcOutput
	if err := json.Unmarshal(stdout.Bytes(), &output); err != nil {
		return nil, err
	}
	shortVersion := versionRegexp.FindString(output.Version)

	// Compilation succeeded, assemble and return the contracts.
	contracts := make(map[string]*Contract)
	for name, info := range output.Contracts {
		// Parse the individual compilation results.
		var abi interface{}
		if err := json.Unmarshal([]byte(info.Abi), &abi); err != nil {
			return nil, fmt.Errorf("solc: error reading abi definition (%v)", err)
		}
		var userdoc interface{}
		if err := json.Unmarshal([]byte(info.Userdoc), &userdoc); err != nil {
			return nil, fmt.Errorf("solc: error reading user doc: %v", err)
		}
		var devdoc interface{}
		if err := json.Unmarshal([]byte(info.Devdoc), &devdoc); err != nil {
			return nil, fmt.Errorf("solc: error reading dev doc: %v", err)
		}
		contracts[name] = &Contract{
			Code: "0x" + info.Bin,
			Info: ContractInfo{
				Source:          source,
				Language:        "Solidity",
				LanguageVersion: shortVersion,
				CompilerVersion: shortVersion,
				CompilerOptions: strings.Join(solcParams, " "),
				AbiDefinition:   abi,
				UserDoc:         userdoc,
				DeveloperDoc:    devdoc,
			},
		}
	}
	return contracts, nil
}

func slurpFiles(files []string) (string, error) {
	var concat bytes.Buffer
	for _, file := range files {
		content, err := ioutil.ReadFile(file)
		if err != nil {
			return "", err
		}
		concat.Write(content)
	}
	return concat.String(), nil
}

func callCSolc(args[] string) bool {
	cargs := C.makeCharArray(C.int(len(args)))
	defer C.freeCharArray(cargs, C.int(len(args)))
	for i, s := range args {
		C.setArrayString(cargs, C.CString(s), C.int(i))
	}

	return 0 == C.solc(len(args), cargs)
}
