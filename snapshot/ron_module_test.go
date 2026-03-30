package snapshot_test

// ronModule represents the parsed RON IR module.
// This is a structural representation that closely matches the RON format.
// Fields use basic Go types (strings, ints, slices) rather than ir.* types
// to avoid coupling the parser to the specific Go IR representation.
// The comparison logic maps between these and ir.Module.
type ronModule struct {
	Types             []ronType
	SpecialTypes      ronSpecialTypes
	Constants         []ronConstant
	Overrides         []ronOverride
	GlobalVariables   []ronGlobalVariable
	GlobalExpressions []ronExpression
	Functions         []ronFunction
	EntryPoints       []ronEntryPoint
	DiagnosticFilters []ronDiagnosticFilter
	DiagFilterLeaf    *uint32
	DocComments       *string
}

type ronSpecialTypes struct {
	RayDesc                   *uint32
	RayIntersection           *uint32
	RayVertexReturn           *uint32
	ExternalTextureParams     *uint32
	ExternalTextureTransferFn *uint32
	PredeclaredTypes          map[string]uint32
}

type ronType struct {
	Name  *string
	Inner ronTypeInner
}

// ronTypeInner is a tagged union for type variants.
type ronTypeInner struct {
	Tag    string // "Scalar", "Vector", "Matrix", "Array", "Struct", "Pointer", "Atomic", "Image", "Sampler", "AccelerationStructure", "RayQuery", "BindingArray"
	Fields map[string]interface{}
}

type ronConstant struct {
	Name *string
	Ty   uint32
	Init uint32 // index into global_expressions
}

type ronOverride struct {
	Name *string
	ID   *uint32
	Ty   uint32
	Init *uint32 // optional index into global_expressions
}

type ronGlobalVariable struct {
	Name    *string
	Space   string // "Storage", "Private", "WorkGroup", "Handle", "Uniform", "PushConstant", "TaskPayload"
	Access  string // for Storage: "LOAD", "LOAD | STORE", etc.
	Binding *ronResourceBinding
	Ty      uint32
	Init    *uint32
}

type ronResourceBinding struct {
	Group   uint32
	Binding uint32
}

type ronFunction struct {
	Name             string // may be "" if anonymous
	Arguments        []ronFunctionArgument
	Result           *ronFunctionResult
	LocalVariables   []ronLocalVariable
	Expressions      []ronExpression
	NamedExpressions map[uint32]string
	Body             []ronStatement
	DiagFilterLeaf   *uint32
}

type ronFunctionArgument struct {
	Name    *string
	Ty      uint32
	Binding *ronBinding
}

type ronFunctionResult struct {
	Ty      uint32
	Binding *ronBinding
}

type ronBinding struct {
	Tag    string // "BuiltIn", "Location"
	Fields map[string]interface{}
}

type ronLocalVariable struct {
	Name *string
	Ty   uint32
	Init *uint32
}

// ronExpression is a tagged union for expression variants.
type ronExpression struct {
	Tag    string
	Fields map[string]interface{}
}

// ronStatement is a tagged union for statement variants.
type ronStatement struct {
	Tag    string
	Fields map[string]interface{}
}

type ronEntryPoint struct {
	Name                   string
	Stage                  string // "Vertex", "Fragment", "Compute", "Task", "Mesh"
	EarlyDepthTest         *string
	WorkgroupSize          [3]uint32
	WorkgroupSizeOverrides *[3]*uint32
	Function               ronFunction
	MeshInfo               *ronMeshInfo
	TaskPayload            *uint32
}

type ronMeshInfo struct {
	Topology              string // "Triangles", "Lines", "Points"
	MaxVertices           uint32
	MaxVerticesOverride   *uint32
	MaxPrimitives         uint32
	MaxPrimitivesOverride *uint32
	VertexOutputType      uint32
	PrimitiveOutputType   uint32
	OutputVariable        uint32
}

type ronDiagnosticFilter struct {
	Inner  ronDiagnosticFilterInner
	Parent *uint32
}

type ronDiagnosticFilterInner struct {
	NewSeverity    string // "Off", "Warning", "Error", "Info"
	TriggeringRule string // e.g., "Standard(DerivativeUniformity)"
}
