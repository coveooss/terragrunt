/*@
  @variables := dict()

  @foreach ($key, $value := my_map)
    @set(variables, $key, $value)
  @end foreach

  @toPrettyHcl(variables)
@*/

