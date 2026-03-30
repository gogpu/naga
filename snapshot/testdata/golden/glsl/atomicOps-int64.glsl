#version 430 core
#extension GL_ARB_compute_shader : require
#extension GL_ARB_shader_storage_buffer_object : require
layout(local_size_x = 2, local_size_y = 1, local_size_z = 1) in;

struct Struct {
    uint atomic_scalar;
    int atomic_arr[2];
};
struct _atomic_compare_exchange_result_Uint_8_ {
    uint64_t old_value;
    bool exchanged;
};
struct _atomic_compare_exchange_result_Sint_8_ {
    int64_t old_value;
    bool exchanged;
};
layout(std430) buffer type_block_0Compute { uint _group_0_binding_0_cs; };

layout(std430) buffer type_2_block_1Compute { int _group_0_binding_1_cs[2]; };

layout(std430) buffer Struct_block_2Compute { Struct _group_0_binding_2_cs; };

shared uint workgroup_atomic_scalar;

shared int workgroup_atomic_arr[2];

shared Struct workgroup_struct;


void main() {
    if (gl_LocalInvocationID == uvec3(0u)) {
        workgroup_atomic_scalar = 0u;
        workgroup_atomic_arr = int[2](0, 0);
        workgroup_struct = Struct(0u, int[2](0, 0));
    }
    memoryBarrierShared();
    barrier();
    uvec3 id = gl_LocalInvocationID;
    _group_0_binding_0_cs = 1uL;
    _group_0_binding_1_cs[1] = 1L;
    _group_0_binding_2_cs.atomic_scalar = 1uL;
    _group_0_binding_2_cs.atomic_arr[1] = 1L;
    workgroup_atomic_scalar = 1uL;
    workgroup_atomic_arr[1] = 1L;
    workgroup_struct.atomic_scalar = 1uL;
    workgroup_struct.atomic_arr[1] = 1L;
    memoryBarrierShared();
    barrier();
    uint64_t l0_ = _group_0_binding_0_cs;
    int64_t l1_ = _group_0_binding_1_cs[1];
    uint64_t l2_ = _group_0_binding_2_cs.atomic_scalar;
    int64_t l3_ = _group_0_binding_2_cs.atomic_arr[1];
    uint64_t l4_ = workgroup_atomic_scalar;
    int64_t l5_ = workgroup_atomic_arr[1];
    uint64_t l6_ = workgroup_struct.atomic_scalar;
    int64_t l7_ = workgroup_struct.atomic_arr[1];
    memoryBarrierShared();
    barrier();
    uint64_t _e51 = atomicAdd(_group_0_binding_0_cs, 1uL);
    int64_t _e55 = atomicAdd(_group_0_binding_1_cs[1], 1L);
    uint64_t _e59 = atomicAdd(_group_0_binding_2_cs.atomic_scalar, 1uL);
    int64_t _e64 = atomicAdd(_group_0_binding_2_cs.atomic_arr[1], 1L);
    uint64_t _e67 = atomicAdd(workgroup_atomic_scalar, 1uL);
    int64_t _e71 = atomicAdd(workgroup_atomic_arr[1], 1L);
    uint64_t _e75 = atomicAdd(workgroup_struct.atomic_scalar, 1uL);
    int64_t _e80 = atomicAdd(workgroup_struct.atomic_arr[1], 1L);
    memoryBarrierShared();
    barrier();
    uint64_t _e83 = atomicAdd(_group_0_binding_0_cs, -1uL);
    int64_t _e87 = atomicAdd(_group_0_binding_1_cs[1], -1L);
    uint64_t _e91 = atomicAdd(_group_0_binding_2_cs.atomic_scalar, -1uL);
    int64_t _e96 = atomicAdd(_group_0_binding_2_cs.atomic_arr[1], -1L);
    uint64_t _e99 = atomicAdd(workgroup_atomic_scalar, -1uL);
    int64_t _e103 = atomicAdd(workgroup_atomic_arr[1], -1L);
    uint64_t _e107 = atomicAdd(workgroup_struct.atomic_scalar, -1uL);
    int64_t _e112 = atomicAdd(workgroup_struct.atomic_arr[1], -1L);
    memoryBarrierShared();
    barrier();
    atomicMax(_group_0_binding_0_cs, 1uL);
    atomicMax(_group_0_binding_1_cs[1], 1L);
    atomicMax(_group_0_binding_2_cs.atomic_scalar, 1uL);
    atomicMax(_group_0_binding_2_cs.atomic_arr[1], 1L);
    atomicMax(workgroup_atomic_scalar, 1uL);
    atomicMax(workgroup_atomic_arr[1], 1L);
    atomicMax(workgroup_struct.atomic_scalar, 1uL);
    atomicMax(workgroup_struct.atomic_arr[1], 1L);
    memoryBarrierShared();
    barrier();
    atomicMin(_group_0_binding_0_cs, 1uL);
    atomicMin(_group_0_binding_1_cs[1], 1L);
    atomicMin(_group_0_binding_2_cs.atomic_scalar, 1uL);
    atomicMin(_group_0_binding_2_cs.atomic_arr[1], 1L);
    atomicMin(workgroup_atomic_scalar, 1uL);
    atomicMin(workgroup_atomic_arr[1], 1L);
    atomicMin(workgroup_struct.atomic_scalar, 1uL);
    atomicMin(workgroup_struct.atomic_arr[1], 1L);
    memoryBarrierShared();
    barrier();
    uint64_t _e163 = atomicAnd(_group_0_binding_0_cs, 1uL);
    int64_t _e167 = atomicAnd(_group_0_binding_1_cs[1], 1L);
    uint64_t _e171 = atomicAnd(_group_0_binding_2_cs.atomic_scalar, 1uL);
    int64_t _e176 = atomicAnd(_group_0_binding_2_cs.atomic_arr[1], 1L);
    uint64_t _e179 = atomicAnd(workgroup_atomic_scalar, 1uL);
    int64_t _e183 = atomicAnd(workgroup_atomic_arr[1], 1L);
    uint64_t _e187 = atomicAnd(workgroup_struct.atomic_scalar, 1uL);
    int64_t _e192 = atomicAnd(workgroup_struct.atomic_arr[1], 1L);
    memoryBarrierShared();
    barrier();
    uint64_t _e195 = atomicOr(_group_0_binding_0_cs, 1uL);
    int64_t _e199 = atomicOr(_group_0_binding_1_cs[1], 1L);
    uint64_t _e203 = atomicOr(_group_0_binding_2_cs.atomic_scalar, 1uL);
    int64_t _e208 = atomicOr(_group_0_binding_2_cs.atomic_arr[1], 1L);
    uint64_t _e211 = atomicOr(workgroup_atomic_scalar, 1uL);
    int64_t _e215 = atomicOr(workgroup_atomic_arr[1], 1L);
    uint64_t _e219 = atomicOr(workgroup_struct.atomic_scalar, 1uL);
    int64_t _e224 = atomicOr(workgroup_struct.atomic_arr[1], 1L);
    memoryBarrierShared();
    barrier();
    uint64_t _e227 = atomicXor(_group_0_binding_0_cs, 1uL);
    int64_t _e231 = atomicXor(_group_0_binding_1_cs[1], 1L);
    uint64_t _e235 = atomicXor(_group_0_binding_2_cs.atomic_scalar, 1uL);
    int64_t _e240 = atomicXor(_group_0_binding_2_cs.atomic_arr[1], 1L);
    uint64_t _e243 = atomicXor(workgroup_atomic_scalar, 1uL);
    int64_t _e247 = atomicXor(workgroup_atomic_arr[1], 1L);
    uint64_t _e251 = atomicXor(workgroup_struct.atomic_scalar, 1uL);
    int64_t _e256 = atomicXor(workgroup_struct.atomic_arr[1], 1L);
    uint64_t _e259 = atomicExchange(_group_0_binding_0_cs, 1uL);
    int64_t _e263 = atomicExchange(_group_0_binding_1_cs[1], 1L);
    uint64_t _e267 = atomicExchange(_group_0_binding_2_cs.atomic_scalar, 1uL);
    int64_t _e272 = atomicExchange(_group_0_binding_2_cs.atomic_arr[1], 1L);
    uint64_t _e275 = atomicExchange(workgroup_atomic_scalar, 1uL);
    int64_t _e279 = atomicExchange(workgroup_atomic_arr[1], 1L);
    uint64_t _e283 = atomicExchange(workgroup_struct.atomic_scalar, 1uL);
    int64_t _e288 = atomicExchange(workgroup_struct.atomic_arr[1], 1L);
    _atomic_compare_exchange_result_Uint_8_ _e292; _e292.old_value = atomicCompSwap(_group_0_binding_0_cs, 1uL, 2uL);
    _e292.exchanged = (_e292.old_value == 1uL);
    _atomic_compare_exchange_result_Sint_8_ _e297; _e297.old_value = atomicCompSwap(_group_0_binding_1_cs[1], 1L, 2L);
    _e297.exchanged = (_e297.old_value == 1L);
    _atomic_compare_exchange_result_Uint_8_ _e302; _e302.old_value = atomicCompSwap(_group_0_binding_2_cs.atomic_scalar, 1uL, 2uL);
    _e302.exchanged = (_e302.old_value == 1uL);
    _atomic_compare_exchange_result_Sint_8_ _e308; _e308.old_value = atomicCompSwap(_group_0_binding_2_cs.atomic_arr[1], 1L, 2L);
    _e308.exchanged = (_e308.old_value == 1L);
    _atomic_compare_exchange_result_Uint_8_ _e312; _e312.old_value = atomicCompSwap(workgroup_atomic_scalar, 1uL, 2uL);
    _e312.exchanged = (_e312.old_value == 1uL);
    _atomic_compare_exchange_result_Sint_8_ _e317; _e317.old_value = atomicCompSwap(workgroup_atomic_arr[1], 1L, 2L);
    _e317.exchanged = (_e317.old_value == 1L);
    _atomic_compare_exchange_result_Uint_8_ _e322; _e322.old_value = atomicCompSwap(workgroup_struct.atomic_scalar, 1uL, 2uL);
    _e322.exchanged = (_e322.old_value == 1uL);
    _atomic_compare_exchange_result_Sint_8_ _e328; _e328.old_value = atomicCompSwap(workgroup_struct.atomic_arr[1], 1L, 2L);
    _e328.exchanged = (_e328.old_value == 1L);
    return;
}

