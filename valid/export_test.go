package valid

import "github.com/gogpu/naga/ir"

// MissingByValidation is the capabilities Validate finds missing with the
// set supported, the oracle MissingCapabilities is tested against: the
// whole validator runs and what its checks remembered as missing is
// returned.
func MissingByValidation(module *ir.Module, supported Capabilities) Capabilities {
	v := &validator{
		module:                 module,
		capabilities:           supported,
		functionStages:         make([]ShaderStages, len(module.Functions)),
		functionSubgroupStages: make([]ShaderStages, len(module.Functions)),
		errors:                 make([]Error, 0),
	}
	v.subgroupStages, v.subgroupOperations = subgroupSets(Options{Capabilities: supported})
	v.validateModule()
	return v.missing
}
