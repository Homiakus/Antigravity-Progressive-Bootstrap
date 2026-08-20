package compiler

import (
	"sort"
	"strings"

	"github.com/homiakus/agctl/internal/harness/ir"
	harnessmodel "github.com/homiakus/agctl/internal/harness/model"
)

func normalize(def ir.Definition) (ir.Definition, error) {
	out := def
	out.Name = strings.TrimSpace(out.Name)
	out.Metadata = cloneStringMap(def.Metadata)
	out.EntryNodes = append([]harnessmodel.NodeID(nil), def.EntryNodes...)
	out.Nodes = cloneNodes(def.Nodes)
	for i := range out.Nodes {
		n := &out.Nodes[i]
		n.Metadata = cloneStringMap(n.Metadata)
		n.Dependencies = append([]harnessmodel.NodeID(nil), n.Dependencies...)
		sort.Slice(n.Dependencies, func(i, j int) bool { return n.Dependencies[i] < n.Dependencies[j] })
		n.Resources.Capabilities = append([]string(nil), n.Resources.Capabilities...)
		sort.Strings(n.Resources.Capabilities)
		n.Policy.RequiredCapabilities = append([]string(nil), n.Policy.RequiredCapabilities...)
		sort.Strings(n.Policy.RequiredCapabilities)
		n.InputRefs = append([]harnessmodel.InputRef(nil), n.InputRefs...)
		n.OutputDeclarations = append([]harnessmodel.OutputDeclaration(nil), n.OutputDeclarations...)
	}
	sort.Slice(out.EntryNodes, func(i, j int) bool { return out.EntryNodes[i] < out.EntryNodes[j] })
	return out, nil
}

func cloneNodes(in []harnessmodel.NodeSpec) []harnessmodel.NodeSpec {
	out := make([]harnessmodel.NodeSpec, len(in))
	copy(out, in)
	for i := range out {
		out[i].Dependencies = append([]harnessmodel.NodeID(nil), in[i].Dependencies...)
		out[i].Resources.Capabilities = append([]string(nil), in[i].Resources.Capabilities...)
		out[i].Policy.RequiredCapabilities = append([]string(nil), in[i].Policy.RequiredCapabilities...)
		out[i].InputRefs = append([]harnessmodel.InputRef(nil), in[i].InputRefs...)
		out[i].OutputDeclarations = append([]harnessmodel.OutputDeclaration(nil), in[i].OutputDeclarations...)
		out[i].Metadata = cloneStringMap(in[i].Metadata)
	}
	return out
}

func cloneStringMap(in map[string]string) map[string]string {
	if in == nil {
		return nil
	}
	out := make(map[string]string, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}
