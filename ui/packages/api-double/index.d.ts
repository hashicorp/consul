export = apiDouble;
/**
 * @param seed - Seed to be passed to use for fake variables
 * @param path - Root path
 * @param reader - Reader function
 * @param $ - Environment object to read env() variables from
 * @param resolve - Resolver function
 */
declare function apiDouble(seed: number, path:string, reader:Function, $:object, resolve:Function): API;

interface API {
  serve(req:any, response:any, cb:Function)
  mutate(cb:(response:any, config:any) => void)
}
