package firmwareevidence

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
)

func DecodeBuildEvidence(reader io.Reader) (BuildEvidence, error) {
	var evidence BuildEvidence
	if err := decodeStrict(reader, func(d *json.Decoder) error { return d.Decode(&evidence) }); err != nil {
		return BuildEvidence{}, fmt.Errorf("decode firmware build evidence: %w", err)
	}
	if err := ValidateBuildEvidence(evidence); err != nil {
		return BuildEvidence{}, err
	}
	return evidence, nil
}

func DecodeRollbackPackage(reader io.Reader) (RollbackPackage, error) {
	var rollback RollbackPackage
	if err := decodeStrict(reader, func(d *json.Decoder) error { return d.Decode(&rollback) }); err != nil {
		return RollbackPackage{}, fmt.Errorf("decode firmware rollback package: %w", err)
	}
	if err := ValidateRollbackPackage(rollback); err != nil {
		return RollbackPackage{}, err
	}
	return rollback, nil
}

func DecodeDependencyLock(reader io.Reader) (DependencyLockDocument, error) {
	var lock DependencyLockDocument
	if err := decodeStrict(reader, func(d *json.Decoder) error { return d.Decode(&lock) }); err != nil {
		return DependencyLockDocument{}, fmt.Errorf("decode firmware dependency lock: %w", err)
	}
	if err := ValidateDependencyLock(lock); err != nil {
		return DependencyLockDocument{}, err
	}
	return lock, nil
}

func BuildEvidenceHash(evidence BuildEvidence) (string, error) {
	if err := ValidateBuildEvidence(evidence); err != nil {
		return "", err
	}
	return canonicalHash(json.Marshal(evidence))
}

func RollbackPackageHash(rollback RollbackPackage) (string, error) {
	if err := ValidateRollbackPackage(rollback); err != nil {
		return "", err
	}
	return canonicalHash(json.Marshal(rollback))
}

func decodeStrict(reader io.Reader, decodeInto func(*json.Decoder) error) error {
	decoder := json.NewDecoder(reader)
	decoder.DisallowUnknownFields()
	if err := decodeInto(decoder); err != nil {
		return err
	}
	var trailing json.RawMessage
	if err := decoder.Decode(&trailing); err != io.EOF {
		if err == nil {
			return fmt.Errorf("trailing JSON value")
		}
		return fmt.Errorf("trailing JSON: %w", err)
	}
	return nil
}

func canonicalHash(encoded []byte, err error) (string, error) {
	if err != nil {
		return "", fmt.Errorf("marshal canonical evidence: %w", err)
	}
	sum := sha256.Sum256(encoded)
	return hex.EncodeToString(sum[:]), nil
}
