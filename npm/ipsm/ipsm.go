// Package ipsm focus on ip set operation
// Copyright 2018 Microsoft. All rights reserved.
// MIT License
package ipsm

import (
	"fmt"
	"math/rand"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"syscall"
	"testing"

	"github.com/Azure/azure-container-networking/log"
	"github.com/Azure/azure-container-networking/npm/metrics"
	"github.com/Azure/azure-container-networking/npm/util"
	npmerr "github.com/Azure/azure-container-networking/npm/util/errors"
)

type IpsEntry struct {
	operationFlag string
	name          string
	set           string
	spec          []string
}

// IpsetManager stores ipset states.
type IpsetManager struct {
	ListMap map[string]*Ipset //tracks all set lists.
	SetMap  map[string]*Ipset //label -> []ip
}

// Ipset represents one ipset entry.
type Ipset struct {
	name       string
	elements   map[string]string // key = ip, value: context associated to the ip like podUid
	referCount int
}

// NewIpset creates a new instance for Ipset object.
func NewIpset(setName string) *Ipset {
	return &Ipset{
		name:     setName,
		elements: make(map[string]string),
	}
}

// NewIpsetManager creates a new instance for IpsetManager object.
func NewIpsetManager() *IpsetManager {
	return &IpsetManager{
		ListMap: make(map[string]*Ipset),
		SetMap:  make(map[string]*Ipset),
	}
}

// Exists checks if an element exists in setMap/listMap.
func (ipsMgr *IpsetManager) Exists(listName string, setName string, kind string) bool {
	m := ipsMgr.SetMap
	if kind == util.IpsetSetListFlag {
		m = ipsMgr.ListMap
	}

	if _, exists := m[listName]; !exists {
		return false
	}

	if _, exists := m[listName].elements[setName]; !exists {
		return false
	}

	return true
}

// SetExists checks if an ipset exists, and returns the type
func (ipsMgr *IpsetManager) SetExists(setName string) (bool, string) {
	_, exists := ipsMgr.SetMap[setName]
	if exists {
		return exists, util.IpsetSetGenericFlag
	}

	_, exists = ipsMgr.ListMap[setName]
	if exists {
		return exists, util.IpsetSetListFlag
	}

	return exists, ""
}

func isNsSet(setName string) bool {
	return !strings.Contains(setName, "-") && !strings.Contains(setName, ":")
}

// CreateList creates an ipset list. npm maintains one setlist per namespace label.
func (ipsMgr *IpsetManager) CreateList(listName string) *npmerr.NPMError {
	if _, exists := ipsMgr.ListMap[listName]; exists {
		return nil
	}

	entry := &IpsEntry{
		name:          listName,
		operationFlag: util.IpsetCreationFlag,
		set:           util.GetHashedName(listName),
		spec:          []string{util.IpsetSetListFlag},
	}
	log.Logf("Creating List: %+v", entry)
	if errCode, err := ipsMgr.Run(entry); err != nil && errCode != 1 {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to create ipset list %s with err: %+v", listName, err)
		return err
	}

	ipsMgr.ListMap[listName] = NewIpset(listName)

	return nil
}

// DeleteList removes an ipset list.
func (ipsMgr *IpsetManager) DeleteList(listName string) error {
	entry := &IpsEntry{
		operationFlag: util.IpsetDestroyFlag,
		set:           util.GetHashedName(listName),
	}

	if errCode, err := ipsMgr.Run(entry); err != nil {
		if errCode == 1 {
			return nil
		}

		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to delete ipset %s %+v with err: %+v", listName, entry, err)
		return err
	}

	delete(ipsMgr.ListMap, listName)

	return nil
}

// AddToList inserts an ipset to an ipset list.
func (ipsMgr *IpsetManager) AddToList(listName string, setName string) *npmerr.NPMError {
	if listName == setName {
		return nil
	}

	//Check if list being added exists in the listmap, if it exists we don't care about the set type
	exists, _ := ipsMgr.SetExists(setName)

	// if set does not exist, then return because the ipset call will fail due to set not existing
	if !exists {
		return npmerr.Errorf("AddToList", false, fmt.Sprintf("Set [%s] does not exist when attempting to add to list [%s]", setName, listName))
	}

	// Check if the list that is being added to exists
	exists, listtype := ipsMgr.SetExists(listName)

	// Make sure that set returned is of list type, otherwise return because we can't add a set to a non setlist type
	if exists && listtype != util.IpsetSetListFlag {
		return npmerr.Errorf("AddToList", false, fmt.Sprintf("Failed to add set [%s] to list [%s], but list is of type [%s]", setName, listName, listtype))
	} else if !exists {
		// if the list doesn't exist, create it
		if err := ipsMgr.CreateList(listName); err != nil {
			return err
		}
	}

	// check if set already exists in the list
	if ipsMgr.Exists(listName, setName, util.IpsetSetListFlag) {
		return nil
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(listName),
		spec:          []string{util.GetHashedName(setName)},
	}

	// add set to list
	if errCode, err := ipsMgr.Run(entry); err != nil && errCode != 1 {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to create ipset rules. rule: %+v with err: %+v", entry, err)
		return err
	}

	ipsMgr.ListMap[listName].elements[setName] = ""

	return nil
}

// DeleteFromList removes an ipset to an ipset list.
func (ipsMgr *IpsetManager) DeleteFromList(listName string, setName string) error {

	//Check if list being added exists in the listmap, if it exists we don't care about the set type
	exists, _ := ipsMgr.SetExists(setName)

	// if set does not exist, then return because the ipset call will fail due to set not existing
	if !exists {
		return fmt.Errorf("Set [%s] does not exist when attempting to delete from list [%s]", setName, listName)
	}

	//Check if list being added exists in the listmap, if it exists we don't care about the set type
	exists, listtype := ipsMgr.SetExists(listName)

	// if set does not exist, then return because the ipset call will fail due to set not existing
	if !exists {
		return fmt.Errorf("Set [%s] does not exist when attempting to add to list [%s]", setName, listName)
	}

	if listtype != util.IpsetSetListFlag {
		return fmt.Errorf("Set [%s] is of the wrong type when attempting to delete list [%s], actual type [%s]", setName, listName, listtype)
	}

	if _, exists := ipsMgr.ListMap[listName]; !exists {
		metrics.SendErrorLogAndMetric(util.IpsmID, "ipset list with name %s not found", listName)
		return nil
	}

	hashedListName, hashedSetName := util.GetHashedName(listName), util.GetHashedName(setName)
	entry := &IpsEntry{
		operationFlag: util.IpsetDeletionFlag,
		set:           hashedListName,
		spec:          []string{hashedSetName},
	}

	if _, err := ipsMgr.Run(entry); err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to delete ipset entry %+v with err: %+v", entry, err)
		return err
	}

	// Now cleanup the cache
	if _, exists := ipsMgr.ListMap[listName].elements[setName]; exists {
		delete(ipsMgr.ListMap[listName].elements, setName)
	}

	if len(ipsMgr.ListMap[listName].elements) == 0 {
		if err := ipsMgr.DeleteList(listName); err != nil {
			metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to delete ipset list %s with err: %+v", listName, err)
			return err
		}
	}

	return nil
}

// CreateSet creates an ipset.
func (ipsMgr *IpsetManager) CreateSet(setName string, spec []string) *npmerr.NPMError {
	timer := metrics.StartNewTimer()

	if _, exists := ipsMgr.SetMap[setName]; exists {
		return nil
	}

	entry := &IpsEntry{
		name:          setName,
		operationFlag: util.IpsetCreationFlag,
		// Use hashed string for set name to avoid string length limit of ipset.
		set:  util.GetHashedName(setName),
		spec: spec,
	}
	log.Logf("Creating Set: %+v", entry)
	if errCode, err := ipsMgr.Run(entry); err != nil && errCode != 1 {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to create ipset with err: %+v", err)
		return err
	}

	ipsMgr.SetMap[setName] = NewIpset(setName)

	metrics.NumIPSets.Inc()
	timer.StopAndRecord(metrics.AddIPSetExecTime)
	metrics.SetIPSetInventory(setName, 0)

	return nil
}

// DeleteSet removes a set from ipset.
func (ipsMgr *IpsetManager) DeleteSet(setName string) *npmerr.NPMError {
	if _, exists := ipsMgr.SetMap[setName]; !exists {
		metrics.SendErrorLogAndMetric(util.IpsmID, "ipset with name %s not found", setName)
		return nil
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetDestroyFlag,
		set:           util.GetHashedName(setName),
	}

	if _, err := ipsMgr.Run(entry); err != nil {

		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to delete ipset %s. Entry: %+v, err: %+v", setName, entry, err)
		return err
	}

	delete(ipsMgr.SetMap, setName)

	metrics.NumIPSets.Dec()
	metrics.NumIPSetEntries.Add(float64(-metrics.GetIPSetInventory(setName)))
	metrics.SetIPSetInventory(setName, 0)

	return nil
}

// AddToSet inserts an ip to an entry in setMap, and creates/updates the corresponding ipset.
func (ipsMgr *IpsetManager) AddToSet(setName, ip, spec, podUid string) *npmerr.NPMError {
	if ipsMgr.Exists(setName, ip, spec) {

		// make sure we have updated the podUid in case it gets changed
		cachedPodUid := ipsMgr.SetMap[setName].elements[ip]
		if cachedPodUid != podUid {
			log.Logf("AddToSet: PodOwner has changed for Ip: %s, setName:%s, Old podUid: %s, new PodUid: %s. Replace context with new PodOwner.",
				ip, setName, cachedPodUid, podUid)

			ipsMgr.SetMap[setName].elements[ip] = podUid
		}

		return nil
	}

	// possible formats
	//192.168.0.1
	//192.168.0.1,tcp:25227
	// todo: handle ip and port with protocol, plus just ip
	// always guaranteed to have ip, not guaranteed to have port + protocol
	ipDetails := strings.Split(ip, ",")
	if len(ipDetails) > 0 && ipDetails[0] == "" {
		return npmerr.Errorf("AddToSet", false, fmt.Sprintf("Failed to add IP to set [%s], the ip to be added was empty, spec: %+v", setName, spec))
	}

	// check if the set exists, ignore the type of the set being added if it exists since the only purpose is to see if it's created or not
	exists, _ := ipsMgr.SetExists(setName)

	if !exists {
		if err := ipsMgr.CreateSet(setName, append([]string{spec})); err != nil {
			return err
		}
	}

	var resultSpec []string
	if strings.Contains(ip, util.IpsetNomatch) {
		ip = strings.Trim(ip, util.IpsetNomatch)
		resultSpec = append([]string{ip, util.IpsetNomatch})
	} else {
		resultSpec = append([]string{ip})
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(setName),
		spec:          resultSpec,
	}

	// todo: check err handling besides error code, corrupt state possible here
	if errCode, err := ipsMgr.Run(entry); err != nil && errCode != 1 {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to create ipset rules. %+v with err: %+v", entry, err)
		return err
	}

	// Stores the podUid as the context for this ip.
	ipsMgr.SetMap[setName].elements[ip] = podUid

	metrics.NumIPSetEntries.Inc()
	metrics.IncIPSetInventory(setName)

	return nil
}

// DeleteFromSet removes an ip from an entry in setMap, and delete/update the corresponding ipset.
func (ipsMgr *IpsetManager) DeleteFromSet(setName, ip, podUid string) *npmerr.NPMError {
	ipSet, exists := ipsMgr.SetMap[setName]
	if !exists {
		log.Logf("ipset with name %s not found", setName)
		return nil
	}

	// possible formats
	//192.168.0.1
	//192.168.0.1,tcp:25227
	// todo: handle ip and port with protocol, plus just ip
	// always guaranteed to have ip, not guaranteed to have port + protocol
	ipDetails := strings.Split(ip, ",")
	if len(ipDetails) > 0 && ipDetails[0] == "" {
		return npmerr.Errorf("DeleteFromSet", false, fmt.Sprintf("Failed to add IP to set [%s], the ip to be added was empty", setName))
	}

	if _, exists := ipsMgr.SetMap[setName].elements[ip]; exists {
		// in case the IP belongs to a new Pod, then ignore this Delete call as this might be stale
		cachedPodUid := ipSet.elements[ip]
		if cachedPodUid != podUid {
			log.Logf("DeleteFromSet: PodOwner has changed for Ip: %s, setName:%s, Old podUid: %s, new PodUid: %s. Ignore the delete as this is stale update",
				ip, setName, cachedPodUid, podUid)

			return nil
		}
	}

	// TODO optimize to not run this command in case cache has already been updated.
	entry := &IpsEntry{
		operationFlag: util.IpsetDeletionFlag,
		set:           util.GetHashedName(setName),
		spec:          append([]string{ip}),
	}

	if errCode, err := ipsMgr.Run(entry); err != nil {
		if errCode == 1 {
			return nil
		}

		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to delete ipset entry. Entry: %+v, err: %+v", entry, err)
		return err
	}

	// Now cleanup the cache
	delete(ipsMgr.SetMap[setName].elements, ip)

	metrics.NumIPSetEntries.Dec()
	metrics.DecIPSetInventory(setName)

	if len(ipsMgr.SetMap[setName].elements) == 0 {
		ipsMgr.DeleteSet(setName)
	}

	return nil
}

// Clean removes all the empty sets & lists under the namespace.
func (ipsMgr *IpsetManager) Clean() error {
	for setName, set := range ipsMgr.SetMap {
		if len(set.elements) > 0 {
			continue
		}

		if err := ipsMgr.DeleteSet(setName); err != nil {
			metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to clean ipset with err: %+v", err)
			return err
		}
	}

	for listName, list := range ipsMgr.ListMap {
		if len(list.elements) > 0 {
			continue
		}

		if err := ipsMgr.DeleteList(listName); err != nil {
			metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to clean ipset list with err: %+v", err)
			return err
		}
	}

	return nil
}

// Destroy completely cleans ipset.
func (ipsMgr *IpsetManager) Destroy() error {
	entry := &IpsEntry{
		operationFlag: util.IpsetFlushFlag,
	}
	if _, err := ipsMgr.Run(entry); err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to flush ipset with err: %+v", err)
		return err
	}

	entry.operationFlag = util.IpsetDestroyFlag
	if _, err := ipsMgr.Run(entry); err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to destroy ipset with err: %+v", err)
		return err
	}

	//TODO set IPSetInventory to 0 for all set names

	return nil
}

// Run execute an ipset command to update ipset.
func (ipsMgr *IpsetManager) Run(entry *IpsEntry) (int, *npmerr.NPMError) {
	cmdName := util.Ipset
	cmdArgs := append([]string{entry.operationFlag, util.IpsetExistFlag, entry.set}, entry.spec...)
	cmdArgs = util.DropEmptyFields(cmdArgs)

	log.Logf("Executing ipset command %s %v", cmdName, cmdArgs)
	_, err := exec.Command(cmdName, cmdArgs...).Output()
	if msg, failed := err.(*exec.ExitError); failed {
		errCode := msg.Sys().(syscall.WaitStatus).ExitStatus()
		if errCode > 0 {
			metrics.SendErrorLogAndMetric(util.IpsmID, "Error: There was an error running command: [%s %v] Stderr: [%v, %s]", cmdName, strings.Join(cmdArgs, " "), err, strings.TrimSuffix(string(msg.Stderr), "\n"))
		}
		er := fmt.Errorf("%s", strings.TrimSuffix(string(msg.Stderr), "\n"))
		npmerr := npmerr.ConvertToNPMError(entry.operationFlag, er, append([]string{cmdName}, cmdArgs...))
		fmt.Println(npmerr)

		return errCode, npmerr
	}

	return 0, nil
}

// Save saves ipset to file.
func (ipsMgr *IpsetManager) Save(configFile string) error {
	if len(configFile) == 0 {
		configFile = util.IpsetConfigFile
	}

	cmd := exec.Command(util.Ipset, util.IpsetSaveFlag, util.IpsetFileFlag, configFile)
	if err := cmd.Start(); err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to save ipset to file with err: %+v", err)
		return err
	}
	cmd.Wait()

	return nil
}

// Restore restores ipset from file.
func (ipsMgr *IpsetManager) Restore(configFile string) error {
	if len(configFile) == 0 {
		configFile = util.IpsetConfigFile
	}

	f, err := os.Stat(configFile)
	if err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to get file %s stat from ipsm.Restore with err: %+v", configFile, err)
		return err
	}

	if f.Size() == 0 {
		if err := ipsMgr.Destroy(); err != nil {
			return err
		}
	}

	cmd := exec.Command(util.Ipset, util.IpsetRestoreFlag, util.IpsetFileFlag, configFile)
	if err := cmd.Start(); err != nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "Error: failed to to restore ipset from file with err: %+v", err)
		return err
	}
	cmd.Wait()

	//TODO based on the set name and number of entries in the config file, update IPSetInventory

	return nil
}

// DestroyNpmIpsets destroys only ipsets created by NPM
func (ipsMgr *IpsetManager) DestroyNpmIpsets() *npmerr.NPMError {

	cmdName := util.Ipset
	cmdArgs := util.IPsetCheckListFlag

	reply, err := exec.Command(cmdName, cmdArgs).Output()
	if msg, failed := err.(*exec.ExitError); failed {
		errCode := msg.Sys().(syscall.WaitStatus).ExitStatus()
		if errCode > 0 {
			metrics.SendErrorLogAndMetric(util.IpsmID, "{DestroyNpmIpsets} Error: There was an error running command: [%s] Stderr: [%v, %s]", cmdName, err, strings.TrimSuffix(string(msg.Stderr), "\n"))
		}

		npmerr := npmerr.ConvertToNPMError(cmdArgs, err, append([]string{cmdName}, cmdArgs))

		return npmerr
	}
	if reply == nil {
		metrics.SendErrorLogAndMetric(util.IpsmID, "{DestroyNpmIpsets} Received empty string from ipset list while destroying azure-npm ipsets")
		return nil
	}

	log.Logf("{DestroyNpmIpsets} Reply from command %s executed is %s", cmdName+" "+cmdArgs, reply)
	re := regexp.MustCompile("Name: (" + util.AzureNpmPrefix + "\\d+)")
	ipsetRegexSlice := re.FindAllSubmatch(reply, -1)

	if len(ipsetRegexSlice) == 0 {
		log.Logf("No Azure-NPM IPsets are found in the Node.")
		return nil
	}

	ipsetLists := make([]string, 0)
	for _, matchedItem := range ipsetRegexSlice {
		if len(matchedItem) == 2 {
			itemString := string(matchedItem[1])
			if strings.Contains(itemString, util.AzureNpmFlag) {
				ipsetLists = append(ipsetLists, itemString)
			}
		}
	}

	if len(ipsetLists) == 0 {
		return nil
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetFlushFlag,
	}

	for _, ipsetName := range ipsetLists {
		entry := &IpsEntry{
			operationFlag: util.IpsetFlushFlag,
			set:           ipsetName,
		}

		if _, err := ipsMgr.Run(entry); err != nil {
			metrics.SendErrorLogAndMetric(util.IpsmID, "{DestroyNpmIpsets} Error: failed to flush ipset %s with err %+v", ipsetName, err)
		}
	}

	for _, ipsetName := range ipsetLists {
		entry.operationFlag = util.IpsetDestroyFlag
		entry.set = ipsetName
		if _, err := ipsMgr.Run(entry); err != nil {
			metrics.SendErrorLogAndMetric(util.IpsmID, "{DestroyNpmIpsets} Error: failed to destroy ipset %s with err %+v", ipsetName, err)
		}
	}

	return nil
}

const letters = "abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"

func GetIPSetName() string {
	b := make([]byte, 8)

	for i := range b {
		b[i] = letters[rand.Intn(len(letters))]
	}

	return "npm-test-" + string(b)
}

// "Set cannot be destroyed: it is in use by a kernel component"
func TestSetCannotBeDestroyed(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	testset1 := GetIPSetName()
	testlist1 := GetIPSetName()

	if err := ipsMgr.CreateSet(testset1, append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("Failed to create set with err %v", err)
	}

	if err := ipsMgr.AddToSet(testset1, fmt.Sprintf("%s", "1.1.1.1"), util.IpsetIPPortHashFlag, "0"); err != nil {
		t.Errorf("Failed to add to set with err %v", err)
	}

	if err := ipsMgr.AddToList(testlist1, testset1); err != nil {
		t.Errorf("Failed to add to list with err %v", err)
	}

	// Delete set and validate set is not exist.
	if err := ipsMgr.DeleteSet(testset1); err != nil {
		if err.ErrID != npmerr.SetCannotBeDestroyedInUseByKernelComponent {
			t.Errorf("Expected to error with ipset in use by kernel component")
		}
	}
}

func TestElemSeparatorSupportsNone(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	testset1 := GetIPSetName()

	if err := ipsMgr.CreateSet(testset1, append([]string{util.IpsetNetHashFlag})); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.CreateSet")
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetTestFlag,
		set:           util.GetHashedName(testset1),
		spec:          append([]string{fmt.Sprintf("10.104.7.252,3000")}),
	}

	if _, err := ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.ElemSeperatorNotSupported {
		t.Errorf("Expected elem seperator error: %+v", err)
	}
}

func TestIPSetWithGivenNameDoesNotExist(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save with err %+v", err)
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore with err %+v", err)
		}
	}()

	testset1 := GetIPSetName()
	testset2 := GetIPSetName()

	entry := &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(testset1),
		spec:          append([]string{util.GetHashedName(testset2)}),
	}

	var err *npmerr.NPMError
	if _, err = ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.SetWithGivenNameDoesNotExist {
		t.Errorf("Expected set to not exist when adding to nonexistent set %+v", err)
	}
}

func TestIPSetWithGivenNameAlreadyExists(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save with err %+v", err)
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore with err %+v", err)
		}
	}()

	testset1 := GetIPSetName()

	entry := &IpsEntry{
		name:          testset1,
		operationFlag: util.IpsetCreationFlag,
		// Use hashed string for set name to avoid string length limit of ipset.
		set:  util.GetHashedName(testset1),
		spec: append([]string{util.IpsetNetHashFlag}),
	}

	if errCode, err := ipsMgr.Run(entry); err != nil && errCode != 1 {
		t.Errorf("Expected err")
	}

	entry = &IpsEntry{
		name:          testset1,
		operationFlag: util.IpsetCreationFlag,
		// Use hashed string for set name to avoid string length limit of ipset.
		set:  util.GetHashedName(testset1),
		spec: append([]string{util.IpsetSetListFlag}),
	}

	if _, err := ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.IPSetWithGivenNameAlreadyExists {
		t.Errorf("Expected error code to match when set does not exist: %+v", err)
	}
}

func TestIPSetSecondElementIsMissingWhenAddingIpWithNoPort(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save with err: %+v", err)
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	testset1 := GetIPSetName()

	spec := append([]string{util.IpsetIPPortHashFlag})
	if err := ipsMgr.CreateSet(testset1, spec); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.CreateSet when creating port set")
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(testset1),
		spec:          append([]string{fmt.Sprintf("%s", "1.1.1.1")}),
	}

	if _, err := ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.SecondElementIsMissing {
		t.Errorf("Expected to fail when adding ip with no port to set that requires port: %+v", err)
	}
}

func TestIPSetMissingSecondMandatoryArgument(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	testset1 := GetIPSetName()

	spec := append([]string{util.IpsetIPPortHashFlag})
	if err := ipsMgr.CreateSet(testset1, spec); err != nil {
		t.Errorf("TestCreateSet failed @ ipsMgr.CreateSet when creating port set")
	}

	entry := &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(testset1),
		spec:          append([]string{}),
	}

	if _, err := ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.MissingSecondMandatoryArgument {
		t.Errorf("Expected to fail when running ipset command with no second argument: %+v", err)
	}
}

func TestIPSetCannotBeAddedAsElementDoesNotExist(t *testing.T) {
	ipsMgr := NewIpsetManager()
	if err := ipsMgr.Save(util.IpsetTestConfigFile); err != nil {
		t.Errorf("TestAddToList failed @ ipsMgr.Save")
	}

	defer func() {
		if err := ipsMgr.Restore(util.IpsetTestConfigFile); err != nil {
			t.Errorf("TestAddToList failed @ ipsMgr.Restore")
		}
	}()

	testset1 := GetIPSetName()
	testset2 := GetIPSetName()

	spec := append([]string{util.IpsetSetListFlag})
	entry := &IpsEntry{
		operationFlag: util.IpsetCreationFlag,
		set:           util.GetHashedName(testset1),
		spec:          spec,
	}

	if _, err := ipsMgr.Run(entry); err != nil {
		t.Errorf("Expected to not fail when creating ipset: %+v", err)
	}

	entry = &IpsEntry{
		operationFlag: util.IpsetAppendFlag,
		set:           util.GetHashedName(testset1),
		spec:          append([]string{util.GetHashedName(testset2)}),
	}

	if _, err := ipsMgr.Run(entry); err == nil || err.ErrID != npmerr.SetToBeAddedDeletedTestedDoesNotExist {
		t.Errorf("Expected to fail when adding set to list and the set doesn't exist: %+v", err)
	}
}
