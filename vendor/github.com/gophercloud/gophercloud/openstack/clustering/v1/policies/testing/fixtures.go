package testing

import (
	"fmt"
	"net/http"
	"testing"
	"time"

	"github.com/gophercloud/gophercloud/openstack/clustering/v1/policies"
	th "github.com/gophercloud/gophercloud/testhelper"
	fake "github.com/gophercloud/gophercloud/testhelper/client"
)

const PolicyListBody1 = `
{
  "policies": [
    {
      "created_at": "2018-04-02T21:43:30.000000",
      "data": {},
      "domain": null,
      "id": "PolicyListBodyID1",
      "name": "delpol",
      "project": "018cd0909fb44cd5bc9b7a3cd664920e",
      "spec": {
        "description": "A policy for choosing victim node(s) from a cluster for deletion.",
        "properties": {
          "criteria": "OLDEST_FIRST",
          "destroy_after_deletion": true,
          "grace_period": 60,
          "reduce_desired_capacity": false
        },
        "type": "senlin.policy.deletion",
        "version": 1
      },
      "type": "senlin.policy.deletion-1.0",
      "updated_at": "2018-04-02T00:19:12Z",
      "user": "fe43e41739154b72818565e0d2580819"
    }
  ]
}
`

const PolicyListBody2 = `
{
  "policies": [
    {
      "created_at": "2018-04-02T22:29:36.000000",
      "data": {},
      "domain": null,
      "id": "PolicyListBodyID2",
      "name": "delpol2",
      "project": "018cd0909fb44cd5bc9b7a3cd664920e",
      "spec": {
        "description": "A policy for choosing victim node(s) from a cluster for deletion.",
        "properties": {
          "criteria": "OLDEST_FIRST",
          "destroy_after_deletion": true,
          "grace_period": 60,
          "reduce_desired_capacity": false
        },
        "type": "senlin.policy.deletion",
        "version": "1.0"
      },
      "type": "senlin.policy.deletion-1.0",
      "updated_at": "2018-04-02T23:15:11.000000",
      "user": "fe43e41739154b72818565e0d2580819"
    }
  ]
}
`

const PolicyCreateBody = `
{
  "policy": {
    "created_at": "2018-04-04T00:18:36Z",
    "data": {},
    "domain": null,
    "id": "b99b3ab4-3aa6-4fba-b827-69b88b9c544a",
    "name": "delpol4",
    "project": "018cd0909fb44cd5bc9b7a3cd664920e",
    "spec": {
      "description": "A policy for choosing victim node(s) from a cluster for deletion.",
      "properties": {
        "hooks": {
          "params": {
            "queue": "zaqar_queue_name"
          },
          "timeout": 180,
          "type": "zaqar"
        }
      },
      "type": "senlin.policy.deletion",
      "version": 1.1
    },
    "type": "senlin.policy.deletion-1.1",
    "updated_at": null,
    "user": "fe43e41739154b72818565e0d2580819"
  }
}
`

const PolicyValidateBody = `
{
  "policy": {
    "created_at": "2018-04-02T21:43:30.000000",
    "data": {},
    "domain": null,
    "id": "b99b3ab4-3aa6-4fba-b827-69b88b9c544a",
    "name": "delpol4",
    "project": "018cd0909fb44cd5bc9b7a3cd664920e",
    "spec": {
      "description": "A policy for choosing victim node(s) from a cluster for deletion.",
      "properties": {
        "hooks": {
          "params": {
            "queue": "zaqar_queue_name"
          },
          "timeout": 180,
          "type": "zaqar"
        }
      },
      "type": "senlin.policy.deletion",
      "version": 1.1
    },
    "type": "senlin.policy.deletion-1.1",
    "updated_at": null,
    "user": "fe43e41739154b72818565e0d2580819"
  }
}
`

const PolicyBadValidateBody = `
{
  "policy": {
    "created_at": "invalid",
    "data": {},
    "domain": null,
    "id": "b99b3ab4-3aa6-4fba-b827-69b88b9c544a",
    "name": "delpol4",
    "project": "018cd0909fb44cd5bc9b7a3cd664920e",
    "spec": {
      "description": "A policy for choosing victim node(s) from a cluster for deletion.",
      "properties": {
        "hooks": {
          "params": {
            "queue": "zaqar_queue_name"
          },
          "timeout": 180,
          "type": "zaqar"
        }
      },
      "type": "senlin.policy.deletion",
      "version": 1.1
    },
    "type": "invalid",
    "updated_at": null,
    "user": "fe43e41739154b72818565e0d2580819"
  }
}
`

const PolicyDeleteRequestID = "req-7328d1b0-9945-456f-b2cd-5166b77d14a8"

var (
	ExpectedPolicyCreatedAt1, _      = time.Parse(time.RFC3339, "2018-04-02T21:43:30.000000Z")
	ExpectedPolicyUpdatedAt1, _      = time.Parse(time.RFC3339, "2018-04-02T00:19:12.000000Z")
	ExpectedPolicyCreatedAt2, _      = time.Parse(time.RFC3339, "2018-04-02T22:29:36.000000Z")
	ExpectedPolicyUpdatedAt2, _      = time.Parse(time.RFC3339, "2018-04-02T23:15:11.000000Z")
	ExpectedCreatePolicyCreatedAt, _ = time.Parse(time.RFC3339, "2018-04-04T00:18:36.000000Z")
	ZeroTime, _                      = time.Parse(time.RFC3339, "1-01-01T00:00:00.000000Z")

	// Policy ID to delete
	PolicyIDtoDelete = "1"

	ExpectedPolicies = [][]policies.Policy{
		{
			{
				CreatedAt: ExpectedPolicyCreatedAt1,
				Data:      map[string]interface{}{},
				Domain:    "",
				ID:        "PolicyListBodyID1",
				Name:      "delpol",
				Project:   "018cd0909fb44cd5bc9b7a3cd664920e",

				Spec: policies.Spec{
					Description: "A policy for choosing victim node(s) from a cluster for deletion.",
					Properties: map[string]interface{}{
						"criteria":                "OLDEST_FIRST",
						"destroy_after_deletion":  true,
						"grace_period":            float64(60),
						"reduce_desired_capacity": false,
					},
					Type:    "senlin.policy.deletion",
					Version: "1.0",
				},
				Type:      "senlin.policy.deletion-1.0",
				User:      "fe43e41739154b72818565e0d2580819",
				UpdatedAt: ExpectedPolicyUpdatedAt1,
			},
		},
		{
			{
				CreatedAt: ExpectedPolicyCreatedAt2,
				Data:      map[string]interface{}{},
				Domain:    "",
				ID:        "PolicyListBodyID2",
				Name:      "delpol2",
				Project:   "018cd0909fb44cd5bc9b7a3cd664920e",

				Spec: policies.Spec{
					Description: "A policy for choosing victim node(s) from a cluster for deletion.",
					Properties: map[string]interface{}{
						"criteria":                "OLDEST_FIRST",
						"destroy_after_deletion":  true,
						"grace_period":            float64(60),
						"reduce_desired_capacity": false,
					},
					Type:    "senlin.policy.deletion",
					Version: "1.0",
				},
				Type:      "senlin.policy.deletion-1.0",
				User:      "fe43e41739154b72818565e0d2580819",
				UpdatedAt: ExpectedPolicyUpdatedAt2,
			},
		},
	}

	ExpectedCreatePolicy = policies.Policy{
		CreatedAt: ExpectedCreatePolicyCreatedAt,
		Data:      map[string]interface{}{},
		Domain:    "",
		ID:        "b99b3ab4-3aa6-4fba-b827-69b88b9c544a",
		Name:      "delpol4",
		Project:   "018cd0909fb44cd5bc9b7a3cd664920e",

		Spec: policies.Spec{
			Description: "A policy for choosing victim node(s) from a cluster for deletion.",
			Properties: map[string]interface{}{
				"hooks": map[string]interface{}{
					"params": map[string]interface{}{
						"queue": "zaqar_queue_name",
					},
					"timeout": float64(180),
					"type":    "zaqar",
				},
			},
			Type:    "senlin.policy.deletion",
			Version: "1.1",
		},
		Type:      "senlin.policy.deletion-1.1",
		User:      "fe43e41739154b72818565e0d2580819",
		UpdatedAt: ZeroTime,
	}

	ExpectedValidatePolicy = policies.Policy{
		CreatedAt: ExpectedPolicyCreatedAt1,
		Data:      map[string]interface{}{},
		Domain:    "",
		ID:        "b99b3ab4-3aa6-4fba-b827-69b88b9c544a",
		Name:      "delpol4",
		Project:   "018cd0909fb44cd5bc9b7a3cd664920e",

		Spec: policies.Spec{
			Description: "A policy for choosing victim node(s) from a cluster for deletion.",
			Properties: map[string]interface{}{
				"hooks": map[string]interface{}{
					"params": map[string]interface{}{
						"queue": "zaqar_queue_name",
					},
					"timeout": float64(180),
					"type":    "zaqar",
				},
			},
			Type:    "senlin.policy.deletion",
			Version: "1.1",
		},
		Type:      "senlin.policy.deletion-1.1",
		User:      "fe43e41739154b72818565e0d2580819",
		UpdatedAt: ZeroTime,
	}
)

func HandlePolicyList(t *testing.T) {
	th.Mux.HandleFunc("/v1/policies", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "GET")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		r.ParseForm()
		marker := r.Form.Get("marker")
		switch marker {
		case "":
			fmt.Fprintf(w, PolicyListBody1)
		case "PolicyListBodyID1":
			fmt.Fprintf(w, PolicyListBody2)
		case "PolicyListBodyID2":
			fmt.Fprintf(w, `{"policies":[]}`)
		default:
			t.Fatalf("Unexpected marker: [%s]", marker)
		}
	})
}

func HandlePolicyCreate(t *testing.T) {
	th.Mux.HandleFunc("/v1/policies", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)

		fmt.Fprintf(w, PolicyCreateBody)
	})
}

func HandlePolicyDelete(t *testing.T) {
	th.Mux.HandleFunc("/v1/policies/"+PolicyIDtoDelete, func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "DELETE")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("X-OpenStack-Request-ID", PolicyDeleteRequestID)
		w.WriteHeader(http.StatusNoContent)
	})
}

func HandlePolicyValidate(t *testing.T) {
	th.Mux.HandleFunc("/v1/policies/validate", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, PolicyValidateBody)
	})
}

func HandleBadPolicyValidate(t *testing.T) {
	th.Mux.HandleFunc("/v1/policies/validate", func(w http.ResponseWriter, r *http.Request) {
		th.TestMethod(t, r, "POST")
		th.TestHeader(t, r, "X-Auth-Token", fake.TokenID)

		w.Header().Add("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)

		fmt.Fprintf(w, PolicyBadValidateBody)
	})
}
