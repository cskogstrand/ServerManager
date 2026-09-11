import triple from '../../../assets/screen-setups/triple-screens-transp.png';
import single from '../../../assets/screen-setups/single-screen-transp.png';
import wide from '../../../assets/screen-setups/wide-screen-transp.png';
import ultrawide from '../../../assets/screen-setups/ultrawide-transp.png';
import vr from '../../../assets/screen-setups/vr-transp.png';
import custom from '../../../assets/screen-setups/custom-transp.png';
export const screenSetups = [
  { id:'triple', name:'Triple screens', image:triple },
  { id:'single', name:'Single (16:9)', image:single },
  { id:'wide', name:'Wide (21:9)', image:wide },
  { id:'ultrawide', name:'Ultrawide (32:9)', image:ultrawide },
  { id:'vr', name:'VR', image:vr },
  { id:'custom', name:'Custom', image:custom },
] as const;
export type ScreenSetup = typeof screenSetups[number]['id'];
