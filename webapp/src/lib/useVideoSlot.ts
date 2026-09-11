import {computed,onBeforeUnmount,ref,watch,type Ref} from 'vue';
// Four wall previews at most. Selected players are owned by the theater/Watch;
// opening the theater unmounts the wall and releases its decoders.
const waiting=ref<symbol[]>([]);
export function useVideoSlot(eligible:Ref<boolean>){
 const id=Symbol();
 const remove=()=>{waiting.value=waiting.value.filter(key=>key!==id)};
 watch(eligible,yes=>{if(yes&&!waiting.value.includes(id))waiting.value.push(id);else if(!yes)remove()},{immediate:true});
 onBeforeUnmount(remove);
 return computed(()=>waiting.value.slice(0,4).includes(id));
}
