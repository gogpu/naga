void exact(int a)
{
    int a_1 = (int)0;

    a_1 = a;
    return;
}

void implicit(float a_2)
{
    float a_3 = (float)0;

    a_3 = a_2;
    return;
}

void implicit_dims(float3 v)
{
    float3 v_1 = (float3)0;

    v_1 = v;
    return;
}

void main_1()
{
    exact(int(1));
    implicit(1.0);
    implicit_dims((1.0).xxx);
    return;
}

void main()
{
    main_1();
    return;
}
