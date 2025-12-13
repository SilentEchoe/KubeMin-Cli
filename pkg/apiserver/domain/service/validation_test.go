package service

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"

	"KubeMin-Cli/pkg/apiserver/config"
	"KubeMin-Cli/pkg/apiserver/domain/spec"
	apisv1 "KubeMin-Cli/pkg/apiserver/interfaces/api/dto/v1"
)

func TestValidationService_TryApplication_ValidConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Version:   "1.0.0",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				NameSpace:     "default",
				Replicas:      1,
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "deploy-step",
				Mode:       "StepByStep",
				Components: []string{"backend"},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.True(t, resp.Valid, "Expected valid application config")
	assert.Empty(t, resp.Errors, "Expected no validation errors")
}

func TestValidationService_TryApplication_InvalidName(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	testCases := []struct {
		name        string
		appName     string
		expectedErr string
	}{
		{"empty name", "", apisv1.ErrCodeMissingRequiredField},
		{"name too short", "a", apisv1.ErrCodeNameTooShort},
		{"invalid characters", "My_App", apisv1.ErrCodeInvalidNameFormat},
		{"starts with hyphen", "-app", apisv1.ErrCodeInvalidNameFormat},
		{"ends with hyphen", "app-", apisv1.ErrCodeInvalidNameFormat},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			req := apisv1.CreateApplicationsRequest{
				Name:      tc.appName,
				Component: []apisv1.CreateComponentRequest{},
			}

			resp := svc.TryApplication(ctx, req)

			assert.False(t, resp.Valid, "Expected invalid application config")
			assert.NotEmpty(t, resp.Errors, "Expected validation errors")

			found := false
			for _, err := range resp.Errors {
				if err.Code == tc.expectedErr && err.Field == "name" {
					found = true
					break
				}
			}
			assert.True(t, found, "Expected error code %s for field 'name'", tc.expectedErr)
		})
	}
}

func TestValidationService_TryApplication_DuplicateComponentName(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
			{
				Name:          "backend", // Duplicate
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to duplicate component")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeDuplicateComponent {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected duplicate component error")
}

func TestValidationService_TryApplication_MissingImage(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "", // Missing image
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing image")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingImage {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing image error")
}

func TestValidationService_TryApplication_InvalidComponentType(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: "invalid-type",
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid component type")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidComponentType {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid component type error")
}

func TestValidationService_TryApplication_InvalidProbeConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Probes: []spec.ProbeTraitsSpec{
						{
							Type: "liveness",
							// Missing probe method (exec, httpGet, or tcpSocket)
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid probe config")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidProbeConfig {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid probe config error")
}

func TestValidationService_TryApplication_ValidProbeConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Probes: []spec.ProbeTraitsSpec{
						{
							Type: "liveness",
							HTTPGet: &spec.HTTPGetProbe{
								Path: "/health",
								Port: 8080,
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have probe-related errors
	for _, err := range resp.Errors {
		assert.NotEqual(t, apisv1.ErrCodeInvalidProbeConfig, err.Code, "Should not have probe config error")
	}
}

func TestValidationService_TryApplication_NestedTraitForbidden(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Sidecar: []spec.SidecarTraitsSpec{
						{
							Name:  "sidecar-1",
							Image: "busybox:latest",
							Traits: spec.Traits{
								// Nested sidecar is forbidden
								Sidecar: []spec.SidecarTraitsSpec{
									{
										Name:  "nested-sidecar",
										Image: "busybox:latest",
									},
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to nested sidecar")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeNestedTraitForbidden {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected nested trait forbidden error")
}

func TestValidationService_TryApplication_WorkflowComponentNotFound(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "deploy-step",
				Mode:       "StepByStep",
				Components: []string{"backend", "non-existent-component"},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to component not found")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeComponentNotFound {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected component not found error")
}

func TestValidationService_TryApplication_InvalidWorkflowMode(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "deploy-step",
				Mode:       "invalid-mode",
				Components: []string{"backend"},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid workflow mode")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidWorkflowMode {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid workflow mode error")
}

func TestValidationService_TryApplication_InvalidStorageType(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Storage: []spec.StorageTraitSpec{
						{
							Type:      "invalid-type",
							MountPath: "/data",
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid storage type")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidStorageType {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid storage type error")
}

func TestValidationService_TryApplication_MissingRBACRules(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					RBAC: []spec.RBACPolicySpec{
						{
							ServiceAccount: "my-sa",
							Rules:          []spec.RBACRuleSpec{}, // Empty rules
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing RBAC rules")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingRBACRules {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing RBAC rules error")
}

func TestValidationService_TryApplication_MissingIngressRoutes(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Ingress: []spec.IngressTraitsSpec{
						{
							Name:   "my-ingress",
							Routes: []spec.IngressRoutes{}, // Empty routes
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing ingress routes")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingIngressRoutes {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing ingress routes error")
}

// ==================== Additional Test Cases ====================

func TestValidationService_TryApplication_CompleteValidConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	// Complete valid configuration with all traits
	staticValue := "production"
	req := apisv1.CreateApplicationsRequest{
		Name:        "demo-app",
		NameSpace:   "default",
		Version:     "1.0.0",
		Project:     "demo-project",
		Description: "A complete demo application",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "app-config",
				ComponentType: config.ConfJob,
				NameSpace:     "default",
				Replicas:      1,
				Properties: apisv1.Properties{
					Conf: map[string]string{
						"database.host": "mysql.default.svc",
						"database.port": "3306",
					},
				},
			},
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "myregistry/backend:v1.0.0",
				NameSpace:     "default",
				Replicas:      2,
				Properties: apisv1.Properties{
					Ports: []spec.Ports{{Port: 8080, Expose: true}},
					Env: map[string]string{
						"APP_ENV": "production",
					},
				},
				Traits: apisv1.Traits{
					Probes: []spec.ProbeTraitsSpec{
						{
							Type: "liveness",
							HTTPGet: &spec.HTTPGetProbe{
								Path: "/healthz",
								Port: 8080,
							},
							InitialDelaySeconds: 30,
							PeriodSeconds:       10,
						},
						{
							Type: "readiness",
							HTTPGet: &spec.HTTPGetProbe{
								Path: "/ready",
								Port: 8080,
							},
							InitialDelaySeconds: 5,
							PeriodSeconds:       5,
						},
					},
					Resources: &spec.ResourceTraitsSpec{
						CPU:    "500m",
						Memory: "512Mi",
					},
					Envs: []spec.SimplifiedEnvSpec{
						{
							Name: "APP_ENV",
							ValueFrom: spec.ValueSource{
								Static: &staticValue,
							},
						},
					},
					Storage: []spec.StorageTraitSpec{
						{
							Type:      "persistent",
							Name:      "data",
							MountPath: "/data",
							TmpCreate: true,
							Size:      "10Gi",
						},
					},
				},
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "config-step",
				Mode:       "StepByStep",
				Components: []string{"app-config"},
			},
			{
				Name:       "deploy-backend",
				Mode:       "DAG",
				Components: []string{"backend"},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.True(t, resp.Valid, "Expected valid application config")
	if !resp.Valid {
		for _, err := range resp.Errors {
			t.Logf("Validation error: field=%s code=%s message=%s", err.Field, err.Code, err.Message)
		}
	}
	assert.Empty(t, resp.Errors, "Expected no validation errors")
}

func TestValidationService_TryApplication_MultipleProbeTypes(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	// Test that multiple probe methods in one probe config is invalid
	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Probes: []spec.ProbeTraitsSpec{
						{
							Type: "liveness",
							HTTPGet: &spec.HTTPGetProbe{
								Path: "/health",
								Port: 8080,
							},
							TCPSocket: &spec.TCPSocketProbe{
								Port: 8080,
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to multiple probe methods")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidProbeConfig {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid probe config error")
}

func TestValidationService_TryApplication_ValidEnvFromConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					EnvFrom: []spec.EnvFromSourceSpec{
						{
							Type:       "configMap",
							SourceName: "app-config",
						},
						{
							Type:       "secret",
							SourceName: "app-secrets",
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have envFrom-related errors
	for _, err := range resp.Errors {
		assert.NotEqual(t, apisv1.ErrCodeInvalidEnvFromType, err.Code, "Should not have envFrom type error")
		assert.NotEqual(t, apisv1.ErrCodeMissingRequiredField, err.Code, "Should not have missing field error for envFrom")
	}
}

func TestValidationService_TryApplication_InvalidEnvFromType(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					EnvFrom: []spec.EnvFromSourceSpec{
						{
							Type:       "invalid-type",
							SourceName: "app-config",
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid envFrom type")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidEnvFromType {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid envFrom type error")
}

func TestValidationService_TryApplication_MissingEnvSourceName(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					EnvFrom: []spec.EnvFromSourceSpec{
						{
							Type:       "configMap",
							SourceName: "", // Missing source name
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing sourceName")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingRequiredField && err.Field == "component[0].traits.envFrom[0].sourceName" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing sourceName error")
}

func TestValidationService_TryApplication_ValidEnvsConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	staticValue := "production"
	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Envs: []spec.SimplifiedEnvSpec{
						{
							Name: "APP_ENV",
							ValueFrom: spec.ValueSource{
								Static: &staticValue,
							},
						},
						{
							Name: "DB_PASSWORD",
							ValueFrom: spec.ValueSource{
								Secret: &spec.SecretSelectorSpec{
									Name: "db-credentials",
									Key:  "password",
								},
							},
						},
						{
							Name: "DB_HOST",
							ValueFrom: spec.ValueSource{
								Config: &spec.ConfigMapSelectorSpec{
									Name: "db-config",
									Key:  "host",
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have envs-related errors
	for _, err := range resp.Errors {
		if err.Field != "" && (err.Field == "component[0].traits.envs[0]" ||
			err.Field == "component[0].traits.envs[1]" ||
			err.Field == "component[0].traits.envs[2]") {
			t.Errorf("Unexpected error for envs: %+v", err)
		}
	}
}

func TestValidationService_TryApplication_InvalidEnvValueSource(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Envs: []spec.SimplifiedEnvSpec{
						{
							Name:      "APP_ENV",
							ValueFrom: spec.ValueSource{}, // No value source specified
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing env value source")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidEnvValueSource {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid env value source error")
}

func TestValidationService_TryApplication_MissingStorageMountPath(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Storage: []spec.StorageTraitSpec{
						{
							Type:      "persistent",
							Name:      "data",
							MountPath: "", // Missing mount path
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing mountPath")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingRequiredField && err.Field == "component[0].traits.storage[0].mountPath" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing mountPath error")
}

func TestValidationService_TryApplication_InvalidStorageSize(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Storage: []spec.StorageTraitSpec{
						{
							Type:      "persistent",
							Name:      "data",
							MountPath: "/data",
							TmpCreate: true,
							Size:      "invalid-size", // Invalid size format
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to invalid storage size")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeInvalidStorageSize {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected invalid storage size error")
}

func TestValidationService_TryApplication_MissingRBACVerbs(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					RBAC: []spec.RBACPolicySpec{
						{
							ServiceAccount: "my-sa",
							Rules: []spec.RBACRuleSpec{
								{
									APIGroups: []string{""},
									Resources: []string{"pods"},
									Verbs:     []string{}, // Empty verbs
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing RBAC verbs")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingRBACVerbs {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing RBAC verbs error")
}

func TestValidationService_TryApplication_ValidRBACConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					RBAC: []spec.RBACPolicySpec{
						{
							ServiceAccount: "pod-reader",
							Rules: []spec.RBACRuleSpec{
								{
									APIGroups: []string{""},
									Resources: []string{"pods"},
									Verbs:     []string{"get", "list", "watch"},
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have RBAC-related errors
	for _, err := range resp.Errors {
		assert.NotEqual(t, apisv1.ErrCodeMissingRBACRules, err.Code, "Should not have missing RBAC rules error")
		assert.NotEqual(t, apisv1.ErrCodeMissingRBACVerbs, err.Code, "Should not have missing RBAC verbs error")
	}
}

func TestValidationService_TryApplication_MissingIngressServiceName(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Ingress: []spec.IngressTraitsSpec{
						{
							Name: "my-ingress",
							Routes: []spec.IngressRoutes{
								{
									Path: "/",
									Backend: spec.IngressRoute{
										ServiceName: "", // Missing service name
										ServicePort: 80,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing service name")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingServiceName {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing service name error")
}

func TestValidationService_TryApplication_ValidIngressConfig(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Ingress: []spec.IngressTraitsSpec{
						{
							Name:             "my-ingress",
							IngressClassName: "nginx",
							Hosts:            []string{"api.example.com"},
							Routes: []spec.IngressRoutes{
								{
									Path: "/v1",
									Backend: spec.IngressRoute{
										ServiceName: "api-service",
										ServicePort: 8080,
									},
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have ingress-related errors
	for _, err := range resp.Errors {
		assert.NotEqual(t, apisv1.ErrCodeMissingIngressRoutes, err.Code, "Should not have missing routes error")
		assert.NotEqual(t, apisv1.ErrCodeMissingServiceName, err.Code, "Should not have missing service name error")
	}
}

func TestValidationService_TryApplication_InitContainerMissingImage(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Init: []spec.InitTraitSpec{
						{
							Name: "init-container",
							Properties: spec.Properties{
								Image: "", // Missing image
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing init container image")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingImage {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing image error for init container")
}

func TestValidationService_TryApplication_SidecarMissingImage(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Sidecar: []spec.SidecarTraitsSpec{
						{
							Name:  "sidecar",
							Image: "", // Missing image
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to missing sidecar image")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingImage {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing image error for sidecar")
}

func TestValidationService_TryApplication_NestedInitForbidden(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
				Traits: apisv1.Traits{
					Init: []spec.InitTraitSpec{
						{
							Name: "init-1",
							Properties: spec.Properties{
								Image: "busybox:latest",
							},
							Traits: spec.Traits{
								// Nested init is forbidden
								Init: []spec.InitTraitSpec{
									{
										Name: "nested-init",
										Properties: spec.Properties{
											Image: "busybox:latest",
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to nested init")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeNestedTraitForbidden {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected nested trait forbidden error")
}

func TestValidationService_TryApplication_WorkflowSubStepComponentNotFound(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "deploy-step",
				Mode:       "StepByStep",
				Components: []string{"backend"},
				SubSteps: []apisv1.CreateWorkflowSubStepRequest{
					{
						Name:       "sub-step",
						Components: []string{"non-existent-component"},
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to component not found in substep")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeComponentNotFound {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected component not found error")
}

func TestValidationService_TryApplication_DuplicateWorkflowStepName(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "deploy-step",
				Mode:       "StepByStep",
				Components: []string{"backend"},
			},
			{
				Name:       "deploy-step", // Duplicate
				Mode:       "DAG",
				Components: []string{"backend"},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to duplicate workflow step name")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeDuplicateWorkflowStep {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected duplicate workflow step error")
}

func TestValidationService_TryApplication_EmptyWorkflowStep(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "backend",
				ComponentType: config.ServerJob,
				Image:         "nginx:latest",
			},
		},
		WorkflowSteps: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "empty-step",
				Mode:       "StepByStep",
				Components: []string{}, // No components and no substeps
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	assert.False(t, resp.Valid, "Expected invalid due to empty workflow step")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeWorkflowStepNoComponent {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected workflow step no component error")
}

func TestValidationService_TryApplication_ConfigTypeNoImageRequired(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	// Config type component should not require an image
	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "app-config",
				ComponentType: config.ConfJob,
				Image:         "", // No image required for config type
				Properties: apisv1.Properties{
					Conf: map[string]string{
						"key": "value",
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have missing image error for config type
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingImage {
			t.Errorf("Should not require image for config type component")
		}
	}
}

func TestValidationService_TryApplication_SecretTypeNoImageRequired(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	// Secret type component should not require an image
	req := apisv1.CreateApplicationsRequest{
		Name:      "my-app",
		NameSpace: "default",
		Component: []apisv1.CreateComponentRequest{
			{
				Name:          "app-secret",
				ComponentType: config.SecretJob,
				Image:         "", // No image required for secret type
				Properties: apisv1.Properties{
					Secret: map[string]string{
						"password": "secret123",
					},
				},
			},
		},
	}

	resp := svc.TryApplication(ctx, req)

	// Should not have missing image error for secret type
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingImage {
			t.Errorf("Should not require image for secret type component")
		}
	}
}

// ==================== TryWorkflow Tests ====================

func TestValidationService_TryWorkflow_EmptyWorkflow(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	req := apisv1.TryWorkflowRequest{
		Name:     "test-workflow",
		Workflow: []apisv1.CreateWorkflowStepRequest{},
	}

	resp := svc.TryWorkflow(ctx, "", req)

	assert.False(t, resp.Valid, "Expected invalid due to empty workflow")
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeMissingRequiredField && err.Field == "workflow" {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected missing workflow error")
}

func TestValidationService_TryWorkflow_ValidWorkflow(t *testing.T) {
	svc := &validationServiceImpl{}
	ctx := context.Background()

	// Note: Without appID, component validation will skip
	req := apisv1.TryWorkflowRequest{
		Name: "test-workflow",
		Workflow: []apisv1.CreateWorkflowStepRequest{
			{
				Name:       "step1",
				Mode:       "StepByStep",
				Components: []string{"backend"},
			},
		},
	}

	// Without appID, no component validation
	resp := svc.TryWorkflow(ctx, "", req)

	// Will have component not found error since no appID provided
	found := false
	for _, err := range resp.Errors {
		if err.Code == apisv1.ErrCodeComponentNotFound {
			found = true
			break
		}
	}
	assert.True(t, found, "Expected component not found error when appID is empty")
}
