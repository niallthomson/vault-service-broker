package main

import (
	"encoding/json"
	"fmt"
)

type Details struct {
	OrganizationName string
	OrganizationGUID string

	SpaceName string
	SpaceGUID string

	ServiceInstanceName string
	ServiceInstanceGUID string
}

func (d *Details) NamesPopulated() bool {
	if d.OrganizationName == "" {
		return false
	}
	if d.SpaceName == "" {
		return false
	}
	if d.ServiceInstanceName == "" {
		return false
	}
	return true
}

type Mount struct {
	AbsolutePath string
	Name         string
	GUID         string
	Type         SecretEngineType
}

func (m *Mount) Path() string {
	if m.AbsolutePath != "" {
		return "/cf/" + m.AbsolutePath
	}
	path := fmt.Sprintf("%s", m.GUID)
	if m.Name != "" {
		path = fmt.Sprintf("%s-%s", m.Name, m.GUID)
	}
	return fmt.Sprintf("/cf/%s/%s", path, m.Type.PathType())
}

func (m *Mount) String() string {
	b, _ := json.Marshal(m)
	return fmt.Sprintf("%s", b)
}

type SecretEngineType string

const (
	KV      SecretEngineType = "generic"
	Transit                  = "transit"
)

func (s SecretEngineType) PathType() string {
	switch s {
	case KV:
		return "secret"
	case Transit:
		return "transit"
	}
	return ""
}
