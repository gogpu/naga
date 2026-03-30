void thing()
{
    return;
}

void with_diagnostic()
{
    return;
}

[numthreads(1, 1, 1)]
void main()
{
    thing();
    with_diagnostic();
    return;
}
