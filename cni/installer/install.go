package main

import (
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	ccn "github.com/Azure/azure-container-networking/cni"
)

const (
	binPerm      = 755
	conflistPerm = 644

	linux   = "linux"
	windows = "windows"
	amd64   = "amd64"

	azureCNIBin             = "azure-vnet"
	azureTelemetryBin       = "azure-vnet-telemetry"
	azureCNSIPAM            = "azure-cns"
	auzureVNETIPAM          = "azure-vnet-ipam"
	conflistExtension       = ".conflist"
	cni                     = "cni"
	multitenancy            = "multitenancy"
	singletenancy           = "singletenancy"
	defaultSrcDirLinux      = "/output/"
	defaultBinDirLinux      = "/opt/cni/bin/"
	defaultConflistDirLinux = "/etc/cni/net.d/"

	envCNIOS                     = "CNI_OS"
	envCNITYPE                   = "CNI_TYPE"
	envCNISourceDir              = "CNI_SRC_DIR"
	envCNIDestinationBinDir      = "CNI_DST_BIN_DIR"
	envCNIDestinationConflistDir = "CNI_DST_CONFLIST_DIR"
	envCNIIPAMType               = "CNI_IPAM_TYPE"
	envCNIExemptBins             = "CNI_EXCEMPT_BINS"
)

type environmentalVariables struct {
	srcDir         string
	dstBinDir      string
	dstConflistDir string
	ipamType       string
	exemptBins     map[string]bool
}

type rawConflist struct {
	Name       string        `json:"name"`
	CniVersion string        `json:"cniVersion"`
	Plugins    []interface{} `json:"plugins"`
}

var (
	version string
)

func main() {
	envs, err := getDirectoriesFromEnv()
	if err != nil {
		fmt.Printf("Failed to get environmental variables with err: %v", err)
		os.Exit(1)
	}

	if _, err := os.Stat(envs.dstBinDir); os.IsNotExist(err) {
		os.MkdirAll(envs.dstBinDir, binPerm)
	}
	if err != nil {
		fmt.Printf("Failed to create destination bin %v directory: %v", envs.dstBinDir, err)
		os.Exit(1)
	}

	if _, err := os.Stat(envs.dstConflistDir); os.IsNotExist(err) {
		os.MkdirAll(envs.dstConflistDir, conflistPerm)
	}
	if err != nil {
		fmt.Printf("Failed to create destination conflist %v directory: %v with err %v", envs.dstConflistDir, envs.dstBinDir, err)
		os.Exit(1)
	}

	binaries, conflists, err := getFiles(envs.srcDir)
	if err != nil {
		fmt.Printf("Failed to get CNI related file paths with err: %v", err)
		os.Exit(1)
	}

	err = copyBinaries(binaries, envs, binPerm)
	if err != nil {
		fmt.Printf("Failed to copy CNI binaries with err: %v", err)
		os.Exit(1)
	}

	for _, conf := range conflists {
		err = modifyConflists(conf, envs, conflistPerm)
		if err != nil {
			fmt.Println(err)
			os.Exit(1)
		}
	}

	if version == "" {
		version = "[No version set]"
	}

	fmt.Printf("Successfully installed Azure CNI %s and binaries to %s and conflist to %s\n", version, envs.dstBinDir, envs.dstConflistDir)
}

func modifyConflists(conflistpath string, envs environmentalVariables, perm os.FileMode) error {
	jsonFile, err := os.Open(conflistpath)
	if err != nil {
		return err
	}
	defer jsonFile.Close()

	var conflist rawConflist
	byteValue, err := ioutil.ReadAll(jsonFile)
	if err != nil {
		return err
	}

	err = json.Unmarshal(byteValue, &conflist)
	if err != nil {
		return err
	}

	// if we need to modify the conflist from env's do it here
	if envs.ipamType != "" {
		confmap, err := modifyConf(conflist.Plugins[0], envs)
		if err != nil {
			return err
		}

		conflist.Plugins[0] = confmap

		pretty, _ := json.MarshalIndent(conflist, "", "  ")
		fmt.Printf("Modified conflist from envs:\n-------\n%+v\n-------\n", string(pretty))
	}

	// get target path
	dstFile := envs.dstConflistDir + filepath.Base(conflistpath)
	filebytes, err := json.MarshalIndent(conflist, "", "\t")
	if err != nil {
		return err
	}

	fmt.Printf("Installing %v...\n", dstFile)
	return ioutil.WriteFile(dstFile, filebytes, perm)
}

func modifyConf(conf interface{}, envs environmentalVariables) (interface{}, error) {
	mapbytes, err := json.Marshal(conf)
	if err != nil {
		return nil, err
	}

	netconfig := ccn.NetworkConfig{}
	if err := json.Unmarshal(mapbytes, &netconfig); err != nil {
		return nil, err
	}

	// change the netconfig from passed envs
	netconfig.Ipam.Type = envs.ipamType

	netconfigbytes, _ := json.Marshal(netconfig)
	var rawConfig interface{}
	if err := json.Unmarshal(netconfigbytes, &rawConfig); err != nil {
		return nil, err
	}

	return rawConfig, nil
}

func getDirectoriesFromEnv() (environmentalVariables, error) {
	osVersion := os.Getenv(envCNIOS)
	cniType := os.Getenv(envCNITYPE)
	srcDirectory := os.Getenv(envCNISourceDir)
	dstBinDirectory := os.Getenv(envCNIDestinationBinDir)
	dstConflistDirectory := os.Getenv(envCNIDestinationConflistDir)
	ipamType := os.Getenv(envCNIIPAMType)
	envCNIExemptBins := os.Getenv(envCNIExemptBins)

	envs := environmentalVariables{
		exemptBins: make(map[string]bool),
	}

	// only allow windows and linux binaries
	if strings.EqualFold(osVersion, linux) || strings.EqualFold(osVersion, windows) {
		osVersion = fmt.Sprintf("%s_%s", osVersion, amd64)
	} else {
		return envs, fmt.Errorf("No target OS version supplied, please set %q env and try again", envCNIOS)
	}

	// get paths for singletenancy and multitenancy
	switch {
	case strings.EqualFold(cniType, multitenancy):
		cniType = fmt.Sprintf("%s-%s", cni, multitenancy)
	case strings.EqualFold(cniType, singletenancy):
		cniType = cni
	default:
		return envs, fmt.Errorf("No CNI type supplied, please set %q env to either %q or %q and try again", envCNITYPE, singletenancy, multitenancy)
	}

	// set the source directory where bins and conflist(s) are
	if srcDirectory == "" {
		srcDirectory = defaultSrcDirLinux
	}
	envs.srcDir = fmt.Sprintf("%s%s/%s/", srcDirectory, osVersion, cniType)

	// set the destination directory to install binaries
	if dstBinDirectory == "" {
		dstBinDirectory = defaultBinDirLinux
	}
	envs.dstBinDir = dstBinDirectory

	// set the destination directory to install conflists
	if dstConflistDirectory == "" {
		dstConflistDirectory = defaultConflistDirLinux
	}
	envs.dstConflistDir = dstConflistDirectory

	// set exempt binaries to skip installing
	// convert to all lower case, strip whitespace, and split on comma
	exempt := strings.Split(strings.Replace(strings.ToLower(envCNIExemptBins), " ", "", -1), ",")
	for _, binName := range exempt {
		envs.exemptBins[binName] = true
	}

	// set custom conflist settings
	envs.ipamType = ipamType

	return envs, nil
}

func getFiles(path string) (binaries []string, conflists []string, err error) {
	err = filepath.Walk(path,
		func(path string, info os.FileInfo, err error) error {
			if err != nil {
				return fmt.Errorf("Failed to traverse path %s with err %s", path, err)
			}

			if !info.IsDir() {
				ext := filepath.Ext(path)
				if ext == conflistExtension {
					conflists = append(conflists, path)
				} else {
					binaries = append(binaries, path)
				}

			}

			return nil
		})

	return
}

func copyBinaries(filePaths []string, envs environmentalVariables, perm os.FileMode) error {
	for _, path := range filePaths {
		fileName := filepath.Base(path)

		if exempt, ok := envs.exemptBins[fileName]; ok && exempt {
			fmt.Printf("Skipping %s, marked as exempt\n", fileName)
		} else {
			err := copyFile(path, envs.dstBinDir+fileName, perm)
			fmt.Printf("Installing %v...\n", envs.dstBinDir+fileName)
			if err != nil {
				return err
			}
		}

	}

	return nil
}

func copyFile(src string, dst string, perm os.FileMode) error {
	data, err := ioutil.ReadFile(src)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(dst, data, perm)
	if err != nil {
		return err
	}

	return nil
}
