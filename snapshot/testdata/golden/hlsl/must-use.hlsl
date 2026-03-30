int use_me()
{
    return int(10);
}

int use_return()
{
    const int _e0 = use_me();
    return _e0;
}

int use_assign_var()
{
    int q = (int)0;

    const int _e0 = use_me();
    q = _e0;
    int _e2 = q;
    return _e2;
}

int use_assign_let()
{
    const int _e0 = use_me();
    return _e0;
}

void use_phony_assign()
{
    const int _e0 = use_me();
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    const int _e0 = use_return();
    const int _e1 = use_assign_var();
    const int _e2 = use_assign_let();
    use_phony_assign();
    return;
}
