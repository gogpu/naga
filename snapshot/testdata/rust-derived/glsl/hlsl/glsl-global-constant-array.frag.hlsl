typedef float ret_Constructarray2_float_[2];
ret_Constructarray2_float_ Constructarray2_float_(float arg0, float arg1) {
    float ret[2] = { arg0, arg1 };
    return ret;
}

static const float array[2] = Constructarray2_float_(1.0, 2.0);

void main_1()
{
    float local[2] = array;

    return;
}

void main()
{
    main_1();
    return;
}
