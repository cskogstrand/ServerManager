import {beforeEach,describe,expect,it,vi} from 'vitest';
import {api} from '@/lib/api';
import {clearDriverStreams,useDriverStreams} from '@/lib/useDriverStreams';
vi.mock('@/lib/api',()=>({api:{get:vi.fn()}}));
const get=vi.mocked(api.get);
const camera={id:'source:8',name:'Room camera',enabled:true,player_url:'https://player.example/embed',capture_url:'rtsp://private/token',recording_configured:true,driver_guid:'',instance_id:null,rig_id:null};
beforeEach(()=>{clearDriverStreams();get.mockReset()});
describe('shared source catalogue',()=>{
 it('keeps standalone and disconnected sources available independently of game presence',async()=>{
  get.mockResolvedValue({sources:[camera,{...camera,id:'driver:9',driver_guid:'offline-account'}]});
  const streams=useDriverStreams();await streams.loadStreams();
  expect(streams.allChannelsFor([]).map(c=>c.key)).toEqual(['source:8','driver:9']);
  expect(streams.healthForGuid('offline-account')).toBe('unknown');
 });
 it('drops an in-flight privileged response after logout',async()=>{
  let finish!:(value:unknown)=>void;get.mockReturnValue(new Promise(resolve=>{finish=resolve}));
  const streams=useDriverStreams();const loading=streams.loadStreams();clearDriverStreams();
  finish({sources:[camera]});await loading;
  expect(streams.sources.value).toEqual([]);expect(streams.loaded.value).toBe(false);
 });
 it('reports unavailable status as unknown after a transport failure',async()=>{
  const streams=useDriverStreams();get.mockResolvedValueOnce({statuses:{'source:8':{status:'live'}}});await streams.refreshHealth();
  get.mockRejectedValueOnce(new Error('network'));await streams.refreshHealth();
  expect(streams.health.value['source:8'].status).toBe('unknown');
 });
});
