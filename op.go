package gorgonia

import (
	"encoding/binary"
	"fmt"
	"hash"
	"hash/fnv"

	"github.com/chewxy/gorgonia/tensor/types"
)

// An Op is a symbolic representation of an operation
// Think of them as functions, taking an input (or multiple), and outputting something
//
// All Ops have type signatures that look like this:
//		OpName :: (Floats a) ⇒ Tensor a → Tensor a → Tensor a
type Op interface {
	/* Graph Building Related Methods */

	// Informs the type of the Op (not the node). This will be used by the type system to infer the final type of the node
	Type() Type

	// returns the output shape as a function of the inputs
	inferShape(Type, ...*Node) (types.Shape, error)

	/* Differentiation related fields */

	// diffWRT indicates if the op is differentiable with regards to the number of inputs
	// returns []bool to indicate which input it is differentiable to
	DiffWRT(int) []bool

	// symbolically differentiates the op
	SymDiff(inputs Nodes, outputNode, gradNode *Node) (Nodes, error)

	/* Machine related */

	// executes the op
	Do(...Value) (Value, error)

	/* Analysis Related Methods */

	// indicates if the Op will return a pointer (allowing possible inplace edits) or by value
	// if it's false, the return value of the Op will be a copy of its input
	returnsPtr() bool

	// Does this op potentially call external (cgo or cuda) functions (thereby requiring extra overhead for Go's trampolining thing)
	callsExtern() bool

	// overwriteInput() is a method which states which input the output will be overwriting.
	// This allows for some efficiency gains as the underlying arrays wouldn't have to be re-allocated.
	// The method returns an int instead of a bool because potentially different operations may be allowed
	// to overwrite certain inputs. For example, consider an operation to increment a value:
	// the IncrementOp would be a unary operator, and assuming we would like to overwrite the input,
	// the retVal of overwriteInput() will be 0 (inputs[0]).
	// -1 is returned if overwriting of input is disallowed
	overwriteInput() int

	/* Other methods */
	WriteHash(h hash.Hash)
	Hashcode() uint32
	fmt.Stringer
}

// A UnaryOp is an Op that takes only one input
type UnaryOp interface {
	Op

	isUnary() bool
}

// A BinaryOp is an Op that takes only two inputs
type BinaryOp interface {
	Op

	isBinary() bool
}

// A NoRetOp is an Op that reads a value, but does not return any value. It's a representation of a not-pure function
type NoRetOp interface {
	Op

	returnsNothing() bool
}

// An AdOp is an Op that supports automatic differentiation.
type AdOp interface {
	Op

	DoDiff(inputs Nodes, output *Node) error
}

// a ReductionOp changes the shape of the node
type ReductionOp interface {
	Op

	IsReduction() bool
}

// IncrDoer increments the toIncr with the result of doing
type IncrDoer interface {
	IncrDo(toIncr Value, inputs ...Value) error
}

// UsePreallocDoer is an op that works when a preallocated value is provided
type UsePreallocDoer interface {
	UsePreallocDo(prealloc Value, inputs ...Value) (Value, error)
}

// UnsafeDoer is an op that will overwrite the underlying value.
type UnsafeDoer interface {
	UnsafeDo(inputs ...Value) (Value, error)
}

// a constant is an unchanging value. I think everyone would know what a constant is
// a constant op is an op that creates a constant. It is also a Value of a constant value
type constant interface {
	Op

	isconstant() bool
	Value() Value
}

type constantScalar struct {
	v Scalar
}

func (c constantScalar) Type() Type                                 { return c.v.Type() }
func (c constantScalar) returnsPtr() bool                           { return false }
func (c constantScalar) callsExtern() bool                          { return false }
func (c constantScalar) overwriteInput() int                        { return -1 }
func (c constantScalar) DiffWRT(i int) []bool                       { return nil }
func (c constantScalar) SymDiff(Nodes, *Node, *Node) (Nodes, error) { return nil, nil }

func (c constantScalar) inferShape(Type, ...*Node) (types.Shape, error) {
	return types.ScalarShape(), nil
}

func (c constantScalar) Do(...Value) (Value, error) { return c.v, nil }
func (c constantScalar) String() string             { return fmt.Sprintf("const %s", c.v) }

func (c constantScalar) WriteHash(h hash.Hash) {
	fmt.Fprintf(h, "const ")
	if err := binary.Write(h, binary.LittleEndian, c.v.t); err != nil {
		panic(err)
	}
	fmt.Fprintf(h, "of %v", c.v)
}

func (c constantScalar) Hashcode() uint32 {
	h := fnv.New32a()
	c.WriteHash(h)
	return h.Sum32()
}

func (c constantScalar) isconstant() bool { return true }
func (c constantScalar) Value() Value     { return c.v }

type constantTensor struct {
	v Tensor
}

func (c constantTensor) Type() Type { return c.v.Type() }

// danger! The only reason why this is the case is because matrices may be too large. copying is costly.
// constants should return value but for the sake of memory, we're going to return pointers
func (c constantTensor) returnsPtr() bool                               { return true }
func (c constantTensor) overwriteInput() int                            { return -1 }
func (c constantTensor) callsExtern() bool                              { return false }
func (c constantTensor) DiffWRT(i int) []bool                           { return nil }
func (c constantTensor) SymDiff(Nodes, *Node, *Node) (Nodes, error)     { return nil, nil }
func (c constantTensor) inferShape(Type, ...*Node) (types.Shape, error) { return c.v.Shape(), nil }
func (c constantTensor) Do(...Value) (Value, error)                     { return c.v, nil }
func (c constantTensor) String() string                                 { return fmt.Sprintf("const %s", c.v.Type()) }

func (c constantTensor) WriteHash(h hash.Hash) {
	fmt.Fprintf(h, "const %v", c.Type())
	fmt.Fprintf(h, "%v", c.v)
}

func (c constantTensor) Hashcode() uint32 {
	h := fnv.New32a()
	c.WriteHash(h)
	return h.Sum32()
}

func (c constantTensor) isconstant() bool { return true }
func (c constantTensor) Value() Value     { return c.v }

/* SHAPE RELATED OPs */
