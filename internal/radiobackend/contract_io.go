package radiobackend

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeContract(reader io.Reader) (Contract, error) {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	var contract Contract
	if err := decoder.Decode(&contract); err != nil {
		return Contract{}, fmt.Errorf("decode radio backend contract: %w", err)
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return Contract{}, fmt.Errorf("decode radio backend contract: trailing JSON value")
		}
		return Contract{}, fmt.Errorf("decode radio backend contract: trailing JSON: %w", err)
	}
	if err := ValidateContract(contract); err != nil {
		return Contract{}, err
	}
	return contract, nil
}

func MarshalCanonical(contract Contract) ([]byte, error) {
	if err := ValidateContract(contract); err != nil {
		return nil, err
	}
	raw, err := json.Marshal(CloneContract(contract))
	if err != nil {
		return nil, fmt.Errorf("marshal radio backend contract: %w", err)
	}
	return raw, nil
}

func ContractHash(contract Contract) (string, error) {
	raw, err := MarshalCanonical(contract)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}

func CloneContract(in Contract) Contract {
	out := in
	out.Ownership.NetworkCoreOwned = append([]string(nil), in.Ownership.NetworkCoreOwned...)
	out.Ownership.BackendOwned = append([]string(nil), in.Ownership.BackendOwned...)
	out.Operations = make([]BoundaryOperation, len(in.Operations))
	for i, operation := range in.Operations {
		out.Operations[i] = operation
		out.Operations[i].RequiredFields = append([]string(nil), operation.RequiredFields...)
	}
	out.Candidates = make([]CandidateEvaluation, len(in.Candidates))
	for i, candidate := range in.Candidates {
		out.Candidates[i] = candidate
		out.Candidates[i].Gates = make([]EvaluationGate, len(candidate.Gates))
		for j, gate := range candidate.Gates {
			out.Candidates[i].Gates[j] = gate
			out.Candidates[i].Gates[j].ArtifactRefs = append([]ArtifactRef(nil), gate.ArtifactRefs...)
		}
	}
	return out
}
