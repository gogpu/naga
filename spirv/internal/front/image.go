package front

import (
	"github.com/gogpu/naga/ir"
	"github.com/gogpu/naga/spirv/internal/spvenums"
)

// LookupSampledImage is Rust naga's LookupSampledImage: the image and sampler
// expressions an OpSampledImage instruction couples. They stay expressions
// rather than an IR value: naga IR has no sampled-image type.
type lookupSampledImage struct {
	image   ir.ExpressionHandle
	sampler ir.ExpressionHandle
}

// samplingFlags records how an image or a sampler of the module is used,
// Rust naga's image::SamplingFlags. A handle used both ways is rejected:
// SPIR-V cannot say which way a later sample means.
type samplingFlags uint8

const (
	// samplingRegular is a plain sample, fetch or gather.
	samplingRegular samplingFlags = 1 << iota
	// samplingComparison is a depth comparison sample.
	samplingComparison
)

// extraCoordinate is the extra coordinate a sampling instruction carries
// beyond the image dimensions, Rust naga's ExtraCoordinate.
type extraCoordinate uint8

const (
	// extraArrayLayer reads the last coordinate as the array layer.
	extraArrayLayer extraCoordinate = iota
	// extraProjection divides the coordinates by the last one.
	extraProjection
	// extraGarbage cuts the coordinate down to the image's dimensions.
	extraGarbage
)

// samplingOptions are the flags one sampling instruction family implies,
// Rust naga's SamplingOptions.
type samplingOptions struct {
	project bool
	compare bool
	gather  bool
}

// canComparisonSample reports whether a type is an image or sampler that
// comparison sampling can change, Rust naga's TypeInner::can_comparison_sample:
// a non-multisampled float sampled image, a sampler, or a binding array of
// them.
func (p *parser) canComparisonSample(ty ir.TypeHandle) bool {
	switch t := p.module.Types[ty].Inner.(type) {
	case ir.ImageType:
		return t.Class == ir.ImageClassSampled && !t.Multisampled && t.SampledKind == ir.ScalarFloat
	case ir.SamplerType:
		return true
	case ir.BindingArrayType:
		return p.canComparisonSample(t.Base)
	}
	return false
}

// patchComparisonType rewrites the type of a global that is sampled both ways
// in the module to the depth-comparison form, as Rust naga's
// image::patch_comparison_type: an image becomes a depth image, a sampler
// becomes a comparison sampler. A handle used only for comparison sampling
// keeps its declared type; one used both ways is an error.
func (p *parser) patchComparisonType(handle ir.GlobalVariableHandle, flags samplingFlags) error {
	if flags&samplingComparison == 0 {
		return nil
	}
	if flags == samplingRegular|samplingComparison {
		return errInconsistentComparisonSampling(handle)
	}
	global := &p.module.GlobalVariables[handle]
	original := p.module.Types[global.Type]
	var inner ir.TypeInner
	switch t := original.Inner.(type) {
	case ir.ImageType:
		t.Class = ir.ImageClassDepth
		inner = t
	case ir.SamplerType:
		t.Comparison = true
		inner = t
	default:
		return errInvalidImage(global.Type)
	}
	global.Type = p.insertType(original.Name, inner)
	return nil
}

// imageExprType returns the type of an expression that names an image, Rust
// naga's BlockContext::get_image_expr_ty: the global's type, the argument's,
// or the base's through an access chain.
func (p *parser) imageExprType(fs *functionState, handle ir.ExpressionHandle) (ir.TypeHandle, error) {
	if int(handle) >= len(fs.fn.Expressions) {
		return 0, errInvalidImageExpression
	}
	switch kind := fs.fn.Expressions[handle].Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(kind.Variable) >= len(p.module.GlobalVariables) {
			return 0, errInvalidImageExpression
		}
		return p.module.GlobalVariables[kind.Variable].Type, nil
	case ir.ExprFunctionArgument:
		if int(kind.Index) >= len(fs.fn.Arguments) {
			return 0, errInvalidImageExpression
		}
		return fs.fn.Arguments[kind.Index].Type, nil
	case ir.ExprAccess:
		return p.imageExprType(fs, kind.Base)
	}
	return 0, errInvalidImageExpression
}

// requiredCoordinateSize returns the vector size an image dimension needs of a
// texture coordinate, and false for the scalar coordinate of a 1D image.
func requiredCoordinateSize(dim ir.ImageDimension) (ir.VectorSize, bool) {
	switch dim {
	case ir.Dim2D:
		return ir.Vec2, true
	case ir.Dim3D, ir.DimCube:
		return ir.Vec3, true
	}
	return 0, false
}

// f32Type is the scalar of every texture coordinate.
var f32Type = ir.ScalarType{Kind: ir.ScalarFloat, Width: 4}

// imageTypeOf returns the image type of an expression, or an error when the
// expression is not an image.
func (p *parser) imageTypeOf(fs *functionState, handle ir.ExpressionHandle) (ir.ImageType, error) {
	ty, err := p.imageExprType(fs, handle)
	if err != nil {
		return ir.ImageType{}, err
	}
	image, ok := p.module.Types[ty].Inner.(ir.ImageType)
	if !ok {
		return ir.ImageType{}, errInvalidImage(ty)
	}
	return image, nil
}

// requiredCoordinateType returns the f32 vector type a coordinate is cut down
// to, Rust naga's required_ty lookup: the type parseTypeImage registered.
func (p *parser) requiredCoordinateType(size ir.VectorSize, hasSize bool) ir.TypeHandle {
	if !hasSize {
		return p.insertType("", f32Type)
	}
	return p.insertType("", ir.VectorType{Size: size, Scalar: f32Type})
}

// extractImageCoordinates separates the texture coordinates from the array
// layer, and divides them by the projection term when the instruction projects,
// as Rust naga's extract_image_coordinates.
func (p *parser) extractImageCoordinates(dim ir.ImageDimension, extra extraCoordinate, base ir.ExpressionHandle, coordinateType ir.TypeHandle, fs *functionState) (ir.ExpressionHandle, *ir.ExpressionHandle, error) {
	var givenSize ir.VectorSize
	hasGivenSize := false
	switch t := p.module.Types[coordinateType].Inner.(type) {
	case ir.ScalarType:
	case ir.VectorType:
		givenSize, hasGivenSize = t.Size, true
	default:
		// Rust naga panics on a coordinate that is neither; untrusted input
		// must not.
		return 0, nil, errInvalidImageCoordinate(coordinateType)
	}

	requiredSize, hasRequiredSize := requiredCoordinateSize(dim)
	requiredType := p.requiredCoordinateType(requiredSize, hasRequiredSize)
	extraIndex := uint32(1)
	if hasRequiredSize {
		extraIndex = uint32(requiredSize)
	}

	switch extra {
	case extraArrayLayer:
		var extracted ir.ExpressionHandle
		if !hasRequiredSize {
			extracted = appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: 0})
		} else {
			components := make([]ir.ExpressionHandle, requiredSize)
			for i := range int(requiredSize) {
				components[i] = appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: uint32(i)})
			}
			extracted = appendExpression(fs.fn, ir.ExprCompose{Type: requiredType, Components: components})
		}
		arrayIndexF := appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: extraIndex})
		width := uint8(4)
		arrayIndex := appendExpression(fs.fn, ir.ExprAs{Expr: arrayIndexF, Kind: ir.ScalarSint, Convert: &width})
		return extracted, &arrayIndex, nil

	case extraProjection:
		projection := appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: extraIndex})
		var divided ir.ExpressionHandle
		if !hasRequiredSize {
			temp := appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: 0})
			divided = appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryDivide, Left: temp, Right: projection})
		} else {
			components := make([]ir.ExpressionHandle, requiredSize)
			for i := range int(requiredSize) {
				temp := appendExpression(fs.fn, ir.ExprAccessIndex{Base: base, Index: uint32(i)})
				components[i] = appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryDivide, Left: temp, Right: projection})
			}
			divided = appendExpression(fs.fn, ir.ExprCompose{Type: requiredType, Components: components})
		}
		return divided, nil, nil

	default: // extraGarbage
		if hasGivenSize == hasRequiredSize && (!hasGivenSize || givenSize == requiredSize) {
			return base, nil, nil
		}
		var cut ir.ExpressionKind
		if !hasRequiredSize {
			cut = ir.ExprAccessIndex{Base: base, Index: 0}
		} else {
			cut = ir.ExprSwizzle{
				Size:    requiredSize,
				Vector:  base,
				Pattern: [4]ir.SwizzleComponent{ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleW},
			}
		}
		return appendExpression(fs.fn, cut), nil, nil
	}
}

// splatToVector broadcasts a scalar to a vector of size, Rust naga's
// Expression::Splat.
func splatToVector(fs *functionState, value ir.ExpressionHandle, size ir.VectorSize) ir.ExpressionHandle {
	return appendExpression(fs.fn, ir.ExprSplat{Size: size, Value: value})
}

// parseImageCouple parses OpSampledImage: the image and sampler expressions it
// couples are recorded for the sampling instructions that use the result.
func (p *parser) parseImageCouple(inst instruction) error {
	if err := inst.expect(5); err != nil {
		return err
	}
	if _, err := p.next(); err != nil { // result type
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	samplerID, err := p.next()
	if err != nil {
		return err
	}
	image, err := p.lookupExpr(imageID)
	if err != nil {
		return err
	}
	sampler, err := p.lookupExpr(samplerID)
	if err != nil {
		return err
	}
	p.lookupSampledImage[resultID] = lookupSampledImage{image: image.handle, sampler: sampler.handle}
	return nil
}

// parseImageUncouple parses OpImage: the result is the image half of a
// sampled-image pair.
func (p *parser) parseImageUncouple(inst instruction, fs *functionState, blockID uint32) error {
	if err := inst.expect(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	sampledImageID, err := p.next()
	if err != nil {
		return err
	}
	sampledImage, ok := p.lookupSampledImage[sampledImageID]
	if !ok {
		return errInvalidID(sampledImageID)
	}
	p.storeExpression(resultID, resultTypeID, blockID, sampledImage.image)
	return nil
}

// parseImageWrite parses OpImageWrite into an ImageStore statement. The value
// is expanded to a four-component vector, as Rust naga's parse_image_write.
func (p *parser) parseImageWrite(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int) error {
	extra, err := inst.expectAtLeast(4)
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	coordinateID, err := p.next()
	if err != nil {
		return err
	}
	valueID, err := p.next()
	if err != nil {
		return err
	}
	// Image operands are not carried into the IR; skip them whole, as Rust
	// naga does.
	if extra != 0 {
		if _, err := p.next(); err != nil {
			return err
		}
		for range int(extra) - 1 {
			if _, err := p.next(); err != nil {
				return err
			}
		}
	}

	image, err := p.lookupExpr(imageID)
	if err != nil {
		return err
	}
	imageType, err := p.imageTypeOf(fs, image.handle)
	if err != nil {
		return err
	}

	coordinate, err := p.lookupExpr(coordinateID)
	if err != nil {
		return err
	}
	coordinateHandle, err := p.getExprHandle(coordinateID, coordinate, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	coordinateType, err := p.typeLookup(coordinate.typeID)
	if err != nil {
		return err
	}
	extraCoordinate := extraGarbage
	if imageType.Arrayed {
		extraCoordinate = extraArrayLayer
	}
	coordinateExpr, arrayIndex, err := p.extractImageCoordinates(imageType.Dim, extraCoordinate, coordinateHandle, coordinateType.handle, fs)
	if err != nil {
		return err
	}

	value, err := p.lookupExpr(valueID)
	if err != nil {
		return err
	}
	valueHandle, err := p.getExprHandle(valueID, value, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	valueType, err := p.typeLookup(value.typeID)
	if err != nil {
		return err
	}
	expanded, err := p.expandStoreValue(valueHandle, valueType.handle, fs)
	if err != nil {
		return err
	}

	flushBeforeStatement(consumer, fs.fn.Expressions, block)
	*block = append(*block, ir.Statement{Kind: ir.StmtImageStore{
		Image:      image.handle,
		Coordinate: coordinateExpr,
		ArrayIndex: arrayIndex,
		Value:      expanded,
	}})
	consumer.startWith(fs.fn.Expressions)
	return nil
}

// expandStoreValue widens the stored value to vec4 the way SPIR-V backends
// write texels, Rust naga's expanded_value in parse_image_write.
func (p *parser) expandStoreValue(value ir.ExpressionHandle, valueType ir.TypeHandle, fs *functionState) (ir.ExpressionHandle, error) {
	switch t := p.module.Types[valueType].Inner.(type) {
	case ir.ScalarType:
		return splatToVector(fs, value, ir.Vec4), nil
	case ir.VectorType:
		switch t.Size {
		case ir.Vec2:
			return appendExpression(fs.fn, ir.ExprSwizzle{
				Size:   ir.Vec4,
				Vector: value,
				Pattern: [4]ir.SwizzleComponent{
					ir.SwizzleX, ir.SwizzleY, ir.SwizzleY, ir.SwizzleY,
				},
			}), nil
		case ir.Vec3:
			return appendExpression(fs.fn, ir.ExprSwizzle{
				Size:   ir.Vec4,
				Vector: value,
				Pattern: [4]ir.SwizzleComponent{
					ir.SwizzleX, ir.SwizzleY, ir.SwizzleZ, ir.SwizzleZ,
				},
			}), nil
		case ir.Vec4:
			return value, nil
		}
	}
	return 0, errInvalidVectorType(valueType)
}

// parseImageLoad parses OpImageFetch and OpImageRead into an ImageLoad
// expression, as Rust naga's parse_image_load. A depth image read as a vector
// is splatted.
func (p *parser) parseImageLoad(inst instruction, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	wordsLeft, err := inst.expectAtLeast(5)
	if err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	coordinateID, err := p.next()
	if err != nil {
		return err
	}

	var imageOperands uint32
	if wordsLeft != 0 {
		wordsLeft--
		if imageOperands, err = p.next(); err != nil {
			return err
		}
	}
	var sample, level *ir.ExpressionHandle
	for imageOperands != 0 {
		bit := imageOperands & -imageOperands
		switch spvenums.ImageOperands(bit) {
		case spvenums.ImageOperandsLod:
			lodID, err := p.next()
			if err != nil {
				return err
			}
			lod, err := p.lookupExpr(lodID)
			if err != nil {
				return err
			}
			handle, err := p.getExprHandle(lodID, lod, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			level = &handle
			wordsLeft--
		case spvenums.ImageOperandsSample:
			sampleID, err := p.next()
			if err != nil {
				return err
			}
			sampleLookup, err := p.lookupExpr(sampleID)
			if err != nil {
				return err
			}
			handle := sampleLookup.handle
			sample = &handle
			wordsLeft--
		default:
			// An operand the IR has no field for: skip the rest, as Rust naga.
			for range wordsLeft {
				if _, err := p.next(); err != nil {
					return err
				}
			}
			imageOperands = 0
			continue
		}
		imageOperands ^= bit
	}

	image, err := p.lookupExpr(imageID)
	if err != nil {
		return err
	}
	imageType, err := p.imageTypeOf(fs, image.handle)
	if err != nil {
		return err
	}

	coordinate, err := p.lookupExpr(coordinateID)
	if err != nil {
		return err
	}
	coordinateHandle, err := p.getExprHandle(coordinateID, coordinate, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	coordinateType, err := p.typeLookup(coordinate.typeID)
	if err != nil {
		return err
	}
	extraCoordinate := extraGarbage
	if imageType.Arrayed {
		extraCoordinate = extraArrayLayer
	}
	coordinateExpr, arrayIndex, err := p.extractImageCoordinates(imageType.Dim, extraCoordinate, coordinateHandle, coordinateType.handle, fs)
	if err != nil {
		return err
	}

	handle := appendExpression(fs.fn, ir.ExprImageLoad{
		Image:      image.handle,
		Coordinate: coordinateExpr,
		ArrayIndex: arrayIndex,
		Sample:     sample,
		Level:      level,
	})
	if imageType.Class == ir.ImageClassDepth {
		// The IR reads a depth image as a scalar; the SPIR-V result may be a
		// vector of it.
		resultType, err := p.typeLookup(resultTypeID)
		if err != nil {
			return err
		}
		if vector, ok := p.module.Types[resultType.handle].Inner.(ir.VectorType); ok {
			handle = splatToVector(fs, handle, vector.Size)
		}
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// parseImageSample parses the OpImageSample*, OpImageGather and
// OpImageDrefGather family into an ImageSample expression, as Rust naga's
// parse_image_sample.
func (p *parser) parseImageSample(inst instruction, options samplingOptions, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	// The words beyond the four base operands: the gather component or the
	// depth reference comes first, then the image operands.
	wordsLeft := int(inst.wc) - 5
	if options.gather || options.compare {
		wordsLeft--
	}
	if wordsLeft < 0 {
		return errInvalidOperandCount(inst.op, inst.wc)
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	sampledImageID, err := p.next()
	if err != nil {
		return err
	}
	coordinateID, err := p.next()
	if err != nil {
		return err
	}
	var componentID, drefID uint32
	hasComponent, hasDref := false, false
	switch {
	case options.gather && !options.compare:
		componentID, err = p.next()
		hasComponent = true
	case options.compare:
		drefID, err = p.next()
		hasDref = true
	}
	if err != nil {
		return err
	}

	var level ir.SampleLevel = ir.SampleLevelAuto{}
	var offset *ir.ExpressionHandle
	var imageOperands uint32
	if wordsLeft > 0 {
		wordsLeft--
		if imageOperands, err = p.next(); err != nil {
			return err
		}
	}
	for imageOperands != 0 {
		bit := imageOperands & -imageOperands
		switch spvenums.ImageOperands(bit) {
		case spvenums.ImageOperandsBias:
			biasID, err := p.next()
			if err != nil {
				return err
			}
			bias, err := p.lookupExpr(biasID)
			if err != nil {
				return err
			}
			handle, err := p.getExprHandle(biasID, bias, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			level = ir.SampleLevelBias{Bias: handle}
			wordsLeft--
		case spvenums.ImageOperandsLod:
			lodID, err := p.next()
			if err != nil {
				return err
			}
			lod, err := p.lookupExpr(lodID)
			if err != nil {
				return err
			}
			lodHandle, err := p.getExprHandle(lodID, lod, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			isDepthImage := false
			if sampledImage, ok := p.lookupSampledImage[sampledImageID]; ok {
				if imageType, err := p.imageTypeOf(fs, sampledImage.image); err == nil {
					isDepthImage = imageType.Class == ir.ImageClassDepth
				}
			}
			switch {
			case options.compare:
				// The reference value selects the comparison; a level would be
				// meaningless, so Rust naga assumes it is zero.
				level = ir.SampleLevelZero{}
			case isDepthImage:
				// The IR samples a depth image at an integer level.
				width := uint8(4)
				level = ir.SampleLevelExact{Level: appendExpression(fs.fn, ir.ExprAs{
					Expr:    lodHandle,
					Kind:    ir.ScalarSint,
					Convert: &width,
				})}
			default:
				level = ir.SampleLevelExact{Level: lodHandle}
			}
			wordsLeft--
		case spvenums.ImageOperandsGrad:
			gradXID, err := p.next()
			if err != nil {
				return err
			}
			gradX, err := p.lookupExpr(gradXID)
			if err != nil {
				return err
			}
			gradXHandle, err := p.getExprHandle(gradXID, gradX, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			gradYID, err := p.next()
			if err != nil {
				return err
			}
			gradY, err := p.lookupExpr(gradYID)
			if err != nil {
				return err
			}
			gradYHandle, err := p.getExprHandle(gradYID, gradY, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			if options.compare {
				level = ir.SampleLevelZero{}
			} else {
				level = ir.SampleLevelGradient{X: gradXHandle, Y: gradYHandle}
			}
			wordsLeft -= 2
		case spvenums.ImageOperandsConstOffset:
			offsetID, err := p.next()
			if err != nil {
				return err
			}
			offsetLookup, err := p.lookupExpr(offsetID)
			if err != nil {
				return err
			}
			handle, err := p.getExprHandle(offsetID, offsetLookup, fs, consumer, block, bodyIdx)
			if err != nil {
				return err
			}
			offset = &handle
			wordsLeft--
		default:
			for range wordsLeft {
				if _, err := p.next(); err != nil {
					return err
				}
			}
			imageOperands = 0
			continue
		}
		imageOperands ^= bit
	}

	sampledImage, ok := p.lookupSampledImage[sampledImageID]
	if !ok {
		return errInvalidID(sampledImageID)
	}
	coordinate, err := p.lookupExpr(coordinateID)
	if err != nil {
		return err
	}
	coordinateHandle, err := p.getExprHandle(coordinateID, coordinate, fs, consumer, block, bodyIdx)
	if err != nil {
		return err
	}
	coordinateType, err := p.typeLookup(coordinate.typeID)
	if err != nil {
		return err
	}

	var gather *ir.SwizzleComponent
	if options.gather {
		component := ir.SwizzleX
		if hasComponent {
			componentLookup, err := p.lookupExpr(componentID)
			if err != nil {
				return err
			}
			value, ok := fs.fn.Expressions[componentLookup.handle].Kind.(ir.ExprConstant)
			if !ok {
				return errInvalidOperand
			}
			literal, ok := p.literalConstantValue(value.Constant)
			if !ok {
				return errInvalidOperand
			}
			switch literal {
			case 0:
				component = ir.SwizzleX
			case 1:
				component = ir.SwizzleY
			case 2:
				component = ir.SwizzleZ
			case 3:
				component = ir.SwizzleW
			default:
				return errInvalidOperand
			}
		}
		level = ir.SampleLevelZero{}
		gather = &component
	}

	samplingBit := samplingRegular
	if options.compare {
		samplingBit = samplingComparison
	}

	imageType, err := p.samplingImageType(fs, sampledImage.image, samplingBit)
	if err != nil {
		return err
	}
	if err := p.recordSampling(fs, sampledImage.sampler, samplingBit); err != nil {
		return err
	}

	extraCoordinate := extraGarbage
	if options.project {
		extraCoordinate = extraProjection
	} else if imageType.Arrayed {
		extraCoordinate = extraArrayLayer
	}
	coordinateExpr, arrayIndex, err := p.extractImageCoordinates(imageType.Dim, extraCoordinate, coordinateHandle, coordinateType.handle, fs)
	if err != nil {
		return err
	}

	var depthRef *ir.ExpressionHandle
	if hasDref {
		dref, err := p.lookupExpr(drefID)
		if err != nil {
			return err
		}
		depth, err := p.getExprHandle(drefID, dref, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		if options.project {
			requiredSize, hasRequiredSize := requiredCoordinateSize(imageType.Dim)
			index := uint32(1)
			if hasRequiredSize {
				index = uint32(requiredSize)
			}
			right := appendExpression(fs.fn, ir.ExprAccessIndex{Base: coordinateHandle, Index: index})
			depth = appendExpression(fs.fn, ir.ExprBinary{Op: ir.BinaryDivide, Left: depth, Right: right})
		}
		depthRef = &depth
	}

	handle := appendExpression(fs.fn, ir.ExprImageSample{
		Image:      sampledImage.image,
		Sampler:    sampledImage.sampler,
		Gather:     gather,
		Coordinate: coordinateExpr,
		ArrayIndex: arrayIndex,
		Offset:     offset,
		Level:      level,
		DepthRef:   depthRef,
	})
	if imageType.Class == ir.ImageClassDepth && depthRef == nil && gather == nil {
		// A depth sample without a reference is a scalar in the IR; the
		// SPIR-V result is a vector of it. A gather is a vector already (a
		// textureGather on a depth image is valid WGSL without a reference
		// component), and Rust naga splats it anyway, which its own typifier
		// then rejects.
		handle = splatToVector(fs, handle, ir.Vec4)
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// samplingImageType returns the type of an image that is about to be sampled,
// recording the sampling kind on the global variable it names. A binding array
// yields its element type, Rust naga's parse_image_sample.
func (p *parser) samplingImageType(fs *functionState, image ir.ExpressionHandle, flags samplingFlags) (ir.ImageType, error) {
	if int(image) >= len(fs.fn.Expressions) {
		return ir.ImageType{}, errInvalidGlobalVar
	}
	switch kind := fs.fn.Expressions[image].Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(kind.Variable) >= len(p.module.GlobalVariables) {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		if _, ok := p.handleSampling[kind.Variable]; ok {
			p.handleSampling[kind.Variable] |= flags
		}
		imageType, ok := p.module.Types[p.module.GlobalVariables[kind.Variable].Type].Inner.(ir.ImageType)
		if !ok {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		return imageType, nil
	case ir.ExprAccess:
		base, ok := fs.fn.Expressions[kind.Base].Kind.(ir.ExprGlobalVariable)
		if !ok || int(base.Variable) >= len(p.module.GlobalVariables) {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		if _, ok := p.handleSampling[base.Variable]; ok {
			p.handleSampling[base.Variable] |= flags
		}
		bindingArray, ok := p.module.Types[p.module.GlobalVariables[base.Variable].Type].Inner.(ir.BindingArrayType)
		if !ok {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		imageType, ok := p.module.Types[bindingArray.Base].Inner.(ir.ImageType)
		if !ok {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		return imageType, nil
	case ir.ExprFunctionArgument:
		// An image passed to a helper: the flags travel with the argument and
		// reach the caller's global at the call (Rust naga's
		// parameter_sampling, patched over the call graph in parse).
		if int(kind.Index) >= len(fs.parameterSampling) || int(kind.Index) >= len(fs.fn.Arguments) {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		fs.parameterSampling[kind.Index] |= flags
		imageType, ok := p.module.Types[fs.fn.Arguments[kind.Index].Type].Inner.(ir.ImageType)
		if !ok {
			return ir.ImageType{}, errInvalidGlobalVar
		}
		return imageType, nil
	}
	return ir.ImageType{}, errInvalidGlobalVar
}

// recordSampling records how the sampler half of a sampled image is used.
func (p *parser) recordSampling(fs *functionState, sampler ir.ExpressionHandle, flags samplingFlags) error {
	if int(sampler) >= len(fs.fn.Expressions) {
		return errInvalidGlobalVar
	}
	switch kind := fs.fn.Expressions[sampler].Kind.(type) {
	case ir.ExprGlobalVariable:
		if int(kind.Variable) >= len(p.module.GlobalVariables) {
			return errInvalidGlobalVar
		}
		if _, ok := p.handleSampling[kind.Variable]; ok {
			p.handleSampling[kind.Variable] |= flags
		}
		return nil
	case ir.ExprAccess:
		base, ok := fs.fn.Expressions[kind.Base].Kind.(ir.ExprGlobalVariable)
		if !ok || int(base.Variable) >= len(p.module.GlobalVariables) {
			return errInvalidGlobalVar
		}
		if _, ok := p.handleSampling[base.Variable]; ok {
			p.handleSampling[base.Variable] |= flags
		}
		return nil
	case ir.ExprFunctionArgument:
		if int(kind.Index) >= len(fs.parameterSampling) {
			return errInvalidGlobalVar
		}
		fs.parameterSampling[kind.Index] |= flags
		return nil
	}
	return errInvalidGlobalVar
}

// propagateParameterSampling carries a function's argument sampling flags to
// the globals behind the arguments at every call site, as Rust naga's
// patch_statements does for Call statements: an image or sampler passed to a
// helper decides how the caller's global is sampled. Module.Functions is
// ordered callees first, so one pass settles every function.
func (p *parser) propagateParameterSampling() error {
	calleeFlags := make([][]samplingFlags, len(p.module.Functions))
	for _, function := range p.lookupFunction {
		if int(function.handle) < len(calleeFlags) {
			calleeFlags[function.handle] = function.parameterSampling
		}
	}
	for index := range p.module.Functions {
		fn := &p.module.Functions[index]
		own := calleeFlags[index]
		if err := p.propagateBlockSampling(fn, &own, calleeFlags); err != nil {
			return err
		}
	}
	return nil
}

// propagateBlockSampling walks one function's statements for calls.
func (p *parser) propagateBlockSampling(fn *ir.Function, own *[]samplingFlags, calleeFlags [][]samplingFlags) error {
	var walk func(block ir.Block) error
	walk = func(block ir.Block) error {
		for _, statement := range block {
			switch kind := statement.Kind.(type) {
			case ir.StmtCall:
				if int(kind.Function) >= len(calleeFlags) {
					continue
				}
				callee := calleeFlags[kind.Function]
				for argIndex, argument := range kind.Arguments {
					if argIndex >= len(callee) || callee[argIndex] == 0 {
						continue
					}
					if int(argument) >= len(fn.Expressions) {
						return errInvalidGlobalVar
					}
					switch expr := fn.Expressions[argument].Kind.(type) {
					case ir.ExprGlobalVariable:
						if _, ok := p.handleSampling[expr.Variable]; ok {
							p.handleSampling[expr.Variable] |= callee[argIndex]
						}
					case ir.ExprFunctionArgument:
						if int(expr.Index) >= len(*own) {
							return errInvalidGlobalVar
						}
						(*own)[expr.Index] |= callee[argIndex]
					default:
						return errInvalidGlobalVar
					}
				}
			case ir.StmtBlock:
				if err := walk(kind.Block); err != nil {
					return err
				}
			case ir.StmtIf:
				if err := walk(kind.Accept); err != nil {
					return err
				}
				if err := walk(kind.Reject); err != nil {
					return err
				}
			case ir.StmtLoop:
				if err := walk(kind.Body); err != nil {
					return err
				}
				if err := walk(kind.Continuing); err != nil {
					return err
				}
			case ir.StmtSwitch:
				for _, caseBlock := range kind.Cases {
					if err := walk(caseBlock.Body); err != nil {
						return err
					}
				}
			}
		}
		return nil
	}
	return walk(fn.Body)
}

// parseImageQuerySize parses OpImageQuerySize and OpImageQuerySizeLod. The
// result is signed in the IR where SPIR-V declares it signed, Rust naga's
// parse_image_query_size.
func (p *parser) parseImageQuerySize(inst instruction, atLevel bool, fs *functionState, consumer *emitter, block *ir.Block, bodyIdx int, blockID uint32) error {
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	var level *ir.ExpressionHandle
	if atLevel {
		levelID, err := p.next()
		if err != nil {
			return err
		}
		lookup, err := p.lookupExpr(levelID)
		if err != nil {
			return err
		}
		handle, err := p.getExprHandle(levelID, lookup, fs, consumer, block, bodyIdx)
		if err != nil {
			return err
		}
		level = &handle
	}
	image, err := p.lookupExpr(imageID)
	if err != nil {
		return err
	}
	return p.storeImageQuery(fs, blockID, resultTypeID, resultID, image.handle, ir.ImageQuerySize{Level: level})
}

// parseImageQueryOther parses OpImageQueryLevels and OpImageQuerySamples.
func (p *parser) parseImageQueryOther(inst instruction, query ir.ImageQuery, fs *functionState, blockID uint32) error {
	if err := inst.expect(4); err != nil {
		return err
	}
	resultTypeID, err := p.next()
	if err != nil {
		return err
	}
	resultID, err := p.next()
	if err != nil {
		return err
	}
	imageID, err := p.next()
	if err != nil {
		return err
	}
	image, err := p.lookupExpr(imageID)
	if err != nil {
		return err
	}
	return p.storeImageQuery(fs, blockID, resultTypeID, resultID, image.handle, query)
}

// storeImageQuery records an ImageQuery expression, cast to signed when the
// SPIR-V result type is signed.
func (p *parser) storeImageQuery(fs *functionState, blockID uint32, resultTypeID, resultID uint32, image ir.ExpressionHandle, query ir.ImageQuery) error {
	resultType, err := p.typeLookup(resultTypeID)
	if err != nil {
		return err
	}
	handle := appendExpression(fs.fn, ir.ExprImageQuery{Image: image, Query: query})
	if _, isSize := query.(ir.ImageQuerySize); isSize {
		// SPIR-V's OpImageQuerySize(Lod) appends the layer count of an arrayed
		// image to the spatial size; the IR's ImageQuerySize is the spatial
		// size alone (Rust naga's typifier and our typifier both type it so),
		// and the layers come from ImageQueryNumLayers. Recompose the extended
		// result the instruction asked for.
		handle = p.extendImageQuerySize(fs, image, resultType.handle, handle)
	}
	if kind, ok := scalarKindOf(p.module.Types[resultType.handle].Inner); ok && kind == ir.ScalarSint {
		width := uint8(4)
		handle = appendExpression(fs.fn, ir.ExprAs{Expr: handle, Kind: ir.ScalarSint, Convert: &width})
	}
	p.storeExpression(resultID, resultTypeID, blockID, handle)
	return nil
}

// extendImageQuerySize returns the spatial size with the array layer count
// appended, the result type OpImageQuerySize(Lod) has for an arrayed image.
func (p *parser) extendImageQuerySize(fs *functionState, image ir.ExpressionHandle, resultType ir.TypeHandle, spatial ir.ExpressionHandle) ir.ExpressionHandle {
	imageType, err := p.imageTypeOf(fs, image)
	if err != nil || !imageType.Arrayed {
		return spatial
	}
	spatialSize := 1 // 1D
	switch imageType.Dim {
	case ir.Dim2D, ir.DimCube:
		spatialSize = 2
	case ir.Dim3D:
		spatialSize = 3
	}
	// Only recompose when the instruction really asked for the extended
	// result; anything else is a module the validator reports.
	components := 1
	switch t := p.module.Types[resultType].Inner.(type) {
	case ir.VectorType:
		components = int(t.Size)
	}
	if components != spatialSize+1 {
		return spatial
	}
	u32 := ir.ScalarType{Kind: ir.ScalarUint, Width: 4}
	elements := make([]ir.ExpressionHandle, 0, spatialSize+1)
	if spatialSize == 1 {
		elements = append(elements, spatial)
	} else {
		for i := range spatialSize {
			elements = append(elements, appendExpression(fs.fn, ir.ExprAccessIndex{Base: spatial, Index: uint32(i)}))
		}
	}
	elements = append(elements, appendExpression(fs.fn, ir.ExprImageQuery{Image: image, Query: ir.ImageQueryNumLayers{}}))
	size := ir.Vec2
	switch spatialSize + 1 {
	case 3:
		size = ir.Vec3
	case 4:
		size = ir.Vec4
	}
	return appendExpression(fs.fn, ir.ExprCompose{
		Type:       p.insertType("", ir.VectorType{Size: size, Scalar: u32}),
		Components: elements,
	})
}

// scalarKindOf returns the scalar kind of a scalar or vector type, and false
// for a type that has none.
func scalarKindOf(inner ir.TypeInner) (ir.ScalarKind, bool) {
	switch t := inner.(type) {
	case ir.ScalarType:
		return t.Kind, true
	case ir.VectorType:
		return t.Scalar.Kind, true
	}
	return 0, false
}
