package module

// MetadataNodeKind identifies the type of a metadata node.
type MetadataNodeKind int

// Metadata node kinds matching LLVM 3.7 metadata system.
const (
	MDString MetadataNodeKind = iota // string value
	MDValue                          // typed value reference
	MDTuple                          // tuple of sub-nodes
)

// MetadataNode represents a single metadata node in the module.
type MetadataNode struct {
	Kind MetadataNodeKind

	// ID is assigned during serialization.
	ID int

	// For MDString: the string value.
	StringValue string

	// For MDValue: the type and constant value.
	ValueType  *Type
	ValueConst *Constant

	// For MDValue referencing a function: the referenced function.
	// When set, ValueConst is ignored and the metadata value record
	// emits [pointer-to-function-type-id, function.ValueID]. This is
	// how DXIL encodes !dx.entryPoints[0][0] = void()* @main.
	ValueFunc *Function

	// For MDTuple: sub-nodes. A nil entry represents a null operand.
	SubNodes []*MetadataNode
}

// NamedMetadataNode represents a named metadata entry (e.g., "dx.version").
type NamedMetadataNode struct {
	// Name is the metadata name (e.g., "dx.version", "dx.shaderModel").
	Name string

	// Operands are the metadata nodes referenced by this named entry.
	Operands []*MetadataNode
}

// AddMetadataString creates a metadata string node and adds it to the module.
func (m *Module) AddMetadataString(s string) *MetadataNode {
	node := &MetadataNode{
		Kind:        MDString,
		ID:          len(m.Metadata),
		StringValue: s,
	}
	m.Metadata = append(m.Metadata, node)
	return node
}

// AddMetadataValue creates a metadata value node and adds it to the module.
func (m *Module) AddMetadataValue(ty *Type, c *Constant) *MetadataNode {
	node := &MetadataNode{
		Kind:       MDValue,
		ID:         len(m.Metadata),
		ValueType:  ty,
		ValueConst: c,
	}
	m.Metadata = append(m.Metadata, node)
	return node
}

// AddMetadataFunc creates a metadata value node that references a function.
//
// In LLVM 3.7 bitcode, a metadata value referencing a function uses the
// function's pointer type and the function's global value ID. This is the
// representation required for !dx.entryPoints[0][0] (the entry function
// pointer) — the DXIL validator dereferences this pointer when walking
// entry points, so it must NOT be a null metadata operand.
//
// Reference: Mesa dxil_get_metadata_func() in dxil_module.c.
func (m *Module) AddMetadataFunc(fn *Function) *MetadataNode {
	ptrTy := m.GetPointerType(fn.FuncType)
	node := &MetadataNode{
		Kind:      MDValue,
		ID:        len(m.Metadata),
		ValueType: ptrTy,
		ValueFunc: fn,
	}
	m.Metadata = append(m.Metadata, node)
	return node
}

// AddMetadataTuple creates a metadata tuple node and adds it to the module.
// Nil entries in subNodes represent null operands.
func (m *Module) AddMetadataTuple(subNodes []*MetadataNode) *MetadataNode {
	node := &MetadataNode{
		Kind:     MDTuple,
		ID:       len(m.Metadata),
		SubNodes: subNodes,
	}
	m.Metadata = append(m.Metadata, node)
	return node
}

// AddNamedMetadata adds a named metadata entry to the module.
func (m *Module) AddNamedMetadata(name string, operands []*MetadataNode) {
	m.NamedMetadata = append(m.NamedMetadata, &NamedMetadataNode{
		Name:     name,
		Operands: operands,
	})
}
