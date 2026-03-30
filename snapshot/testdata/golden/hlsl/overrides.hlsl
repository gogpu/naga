static const bool has_point_light = true;
static const float specular_param = 2.3;
static const float gain = 0.0;
static const float width = 0.0;
static const float depth = 0.0;
static const float height = 0.0;
static const float inferred_f32 = 2.718;
static const uint auto_conversion = 0u;

static float gain_x_10;
static float store_override;

[numthreads(1, 1, 1)]
void main()
{
    float t = (height * 5.0);
    bool x;
    float gain_x_100;
    
    float _e2 = (height * 5.0);
    bool a = !(has_point_light);
    x = a;
    float _e7 = gain_x_10;
    float _e9 = (_e7 * 10.0);
    gain_x_100 = _e9;
    store_override = gain;
    return;
}
