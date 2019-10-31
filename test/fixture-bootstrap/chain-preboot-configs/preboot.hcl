/*@
  @variables := dict()

  @foreach($key, $value := .my_map)
    @variables[$key] = $value
  @end foreach

  @toPrettyHcl(variables)
@*/

