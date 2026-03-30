int classify(int x)
{
    int result = (int)0;

    switch(x) {
        case 0: {
            result = int(0);
            break;
        }
        case 1:
        case 2: {
            result = int(1);
            break;
        }
        case 3:
        case 4:
        case 5: {
            result = int(2);
            break;
        }
        default: {
            result = int(-1);
            break;
        }
    }
    int _e6 = result;
    return _e6;
}

uint switch_with_return(uint x_1)
{
    switch(x_1) {
        case 0u: {
            return 100u;
        }
        case 1u: {
            return 200u;
        }
        default: {
            return 0u;
        }
    }
}

int nested_switch(int a, int b)
{
    int result_1 = int(0);

    switch(a) {
        case 1: {
            switch(b) {
                case 10: {
                    result_1 = int(110);
                    break;
                }
                default: {
                    result_1 = int(100);
                    break;
                }
            }
            break;
        }
        default: {
            result_1 = int(0);
            break;
        }
    }
    int _e7 = result_1;
    return _e7;
}

[numthreads(1, 1, 1)]
void main()
{
    const int _e1 = classify(int(3));
    const uint _e3 = switch_with_return(1u);
    const int _e6 = nested_switch(int(1), int(10));
    return;
}
