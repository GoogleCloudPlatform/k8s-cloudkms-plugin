// Copyright 2019 Google, LLC.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

// Package tpm implements TPM2 API to seal and Unseal data.
package tpm

import (
	"fmt"
	"io"

	"github.com/golang/glog"
	"github.com/google/go-tpm/tpm2"
	"github.com/google/go-tpm/tpmutil"
)

var (
	// Default EK template defined in:
	// https://trustedcomputinggroup.org/wp-content/uploads/Credential_Profile_EK_V2.0_R14_published.pdf
	// Shared SRK template based off of EK template and specified in:
	// https://trustedcomputinggroup.org/wp-content/uploads/TCG-TPM-v2.0-Provisioning-Guidance-Published-v1r1.pdf
	srkTemplate = tpm2.Public{
		Type:       tpm2.AlgRSA,
		NameAlg:    tpm2.AlgSHA256,
		Attributes: tpm2.FlagFixedTPM | tpm2.FlagFixedParent | tpm2.FlagSensitiveDataOrigin | tpm2.FlagUserWithAuth | tpm2.FlagRestricted | tpm2.FlagDecrypt | tpm2.FlagNoDA,
		AuthPolicy: nil,
		RSAParameters: &tpm2.RSAParams{
			Symmetric: &tpm2.SymScheme{
				Alg:     tpm2.AlgAES,
				KeyBits: 128,
				Mode:    tpm2.AlgCFB,
			},
			KeyBits:     2048,
			ExponentRaw: 0,
			ModulusRaw:  make([]byte, 256),
		},
	}
)

// Seal seals supplied data to TPM using PCRs and password.
func Seal(tpmPath string, pcr int, srkPassword, objectPassword string, dataToSeal []byte) ([]byte, []byte, error) {
	rwc, err := tpm2.OpenTPM(tpmPath)
	if err != nil {
		return nil, nil, fmt.Errorf("can't open TPM %q: %v", tpmPath, err)
	}
	defer rwc.Close()

	// Create the parent key against which to seal the data
	srkHandle, _, err := tpm2.CreatePrimary(rwc, tpm2.HandleOwner, tpm2.PCRSelection{}, "", srkPassword, srkTemplate)
	if err != nil {
		return nil, nil, fmt.Errorf("can't create primary key: %v", err)
	}
	defer tpm2.FlushContext(rwc, srkHandle)

	glog.Infof("Created parent key with handle: 0x%x\n", srkHandle)

	// Note the value of the pcr against which we will seal the data
	pcrVal, err := tpm2.ReadPCR(rwc, pcr, tpm2.AlgSHA256)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to read PCR: %v", err)
	}
	glog.Infof("PCR %v value: 0x%x\n", pcr, pcrVal)

	// Get the authorization policy that will protect the data to be sealed
	sessHandle, policy, err := policyPCRPasswordSession(rwc, pcr, objectPassword)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to get policy: %v", err)
	}
	if err := tpm2.FlushContext(rwc, sessHandle); err != nil {
		return nil, nil, fmt.Errorf("unable to flush session: %v", err)
	}
	glog.Infof("Created authorization policy: 0x%x\n", policy)

	// Seal the data to the parent key and the policy
	privateArea, publicArea, err := tpm2.Seal(rwc, srkHandle, srkPassword, objectPassword, policy, dataToSeal)
	if err != nil {
		return nil, nil, fmt.Errorf("unable to seal data: %v", err)
	}
	glog.Infof("Sealed data: 0x%x\n", privateArea)

	return privateArea, publicArea, nil
}

// Unseal unseals data from TPM based on PCRs and password defined by the polic attached to the protected object.
func Unseal(tpmPath string, pcr int, srkPassword, objectPassword string, privateArea, publicArea []byte) ([]byte, error) {
	rwc, err := tpm2.OpenTPM(tpmPath)
	if err != nil {
		return nil, fmt.Errorf("can't open TPM %q: %v", tpmPath, err)
	}
	defer rwc.Close()

	// Create the parent key against which to seal the data
	srkHandle, _, err := tpm2.CreatePrimary(rwc, tpm2.HandleOwner, tpm2.PCRSelection{}, "", srkPassword, srkTemplate)
	if err != nil {
		return nil, fmt.Errorf("can't create primary key: %v", err)
	}
	defer tpm2.FlushContext(rwc, srkHandle)

	glog.Infof("Created parent key with handle: 0x%x\n", srkHandle)

	// Load the sealed data into the TPM.
	objectHandle, _, err := tpm2.Load(rwc, srkHandle, srkPassword, publicArea, privateArea)
	if err != nil {
		return nil, fmt.Errorf("unable to load data: %v", err)
	}
	defer tpm2.FlushContext(rwc, objectHandle)

	glog.Infof("Loaded sealed data with handle: 0x%x\n", objectHandle)

	// Create the authorization session
	sessHandle, _, err := policyPCRPasswordSession(rwc, pcr, objectPassword)
	if err != nil {
		return nil, fmt.Errorf("unable to get auth session: %v", err)
	}
	defer tpm2.FlushContext(rwc, sessHandle)

	// Unseal the data
	unsealedData, err := tpm2.UnsealWithSession(rwc, sessHandle, objectHandle, objectPassword)
	if err != nil {
		return nil, fmt.Errorf("unable to Unseal data: %v", err)
	}
	return unsealedData, nil
}

// Returns session handle and policy digest.
func policyPCRPasswordSession(rwc io.ReadWriteCloser, pcr int, password string) (tpmutil.Handle, []byte, error) {
	// FYI, this is not a very secure session.
	sessHandle, _, err := tpm2.StartAuthSession(
		rwc,
		tpm2.HandleNull,  /*tpmKey*/
		tpm2.HandleNull,  /*bindKey*/
		make([]byte, 16), /*nonceCaller*/
		nil,              /*secret*/
		tpm2.SessionPolicy,
		tpm2.AlgNull,
		tpm2.AlgSHA256)
	if err != nil {
		return tpm2.HandleNull, nil, fmt.Errorf("unable to start session: %v", err)
	}

	pcrSelection := tpm2.PCRSelection{
		Hash: tpm2.AlgSHA256,
		PCRs: []int{pcr},
	}

	// An empty expected digest means that digest verification is skipped.
	if err := tpm2.PolicyPCR(rwc, sessHandle, nil /*expectedDigest*/, pcrSelection); err != nil {
		return sessHandle, nil, fmt.Errorf("unable to bind PCRs to auth policy: %v", err)
	}

	if err := tpm2.PolicyPassword(rwc, sessHandle); err != nil {
		return sessHandle, nil, fmt.Errorf("unable to require password for auth policy: %v", err)
	}

	policy, err := tpm2.PolicyGetDigest(rwc, sessHandle)
	if err != nil {
		return sessHandle, nil, fmt.Errorf("unable to get policy digest: %v", err)
	}
	return sessHandle, policy, nil
}
