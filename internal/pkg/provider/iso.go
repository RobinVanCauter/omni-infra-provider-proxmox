// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package provider

import (
	"fmt"
	"strings"

	"github.com/google/cel-go/cel"
	"github.com/luthermonson/go-proxmox"
	siderocel "github.com/siderolabs/talos/pkg/machinery/cel"
)

func pickISOStorageFromList(nodeName string, storages proxmox.Storages, selector string) (*proxmox.Storage, error) {
	if selector == "" {
		for _, storage := range storages {
			if storage == nil || storage.Enabled == 0 || !strings.Contains(storage.Content, "iso") {
				continue
			}

			return storage, nil
		}

		return nil, fmt.Errorf("failed to pick ISO storage: no enabled ISO-capable storage available on node %q", nodeName)
	}

	env, err := cel.NewEnv(
		cel.Variable("name", cel.StringType),
		cel.Variable("node", cel.StringType),
		cel.Variable("storageType", cel.StringType),
		cel.Variable("availableSpace", cel.UintType),
	)
	if err != nil {
		return nil, err
	}

	expr, err := siderocel.ParseBooleanExpression(selector, env)
	if err != nil {
		return nil, err
	}

	for _, storage := range storages {
		if storage == nil || storage.Enabled == 0 || !strings.Contains(storage.Content, "iso") {
			continue
		}

		matched, evalErr := expr.EvalBool(env, map[string]any{
			"name":           storage.Name,
			"node":           nodeName,
			"storageType":    storage.Type,
			"availableSpace": storage.Avail,
		})
		if evalErr != nil {
			return nil, evalErr
		}

		if matched {
			return storage, nil
		}
	}

	return nil, fmt.Errorf("failed to pick ISO storage: no enabled ISO-capable storage matches the condition %q on node %q", selector, nodeName)
}
