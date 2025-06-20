// import Peer from 'simple-peer'
// import { v4 as uuidv4 } from 'uuid'
//
// let peerInstance = null
//
// export async function connectWebRTCPeer() {
//     const offerRes = await fetch('/rtc/offer', { method: 'POST' }).then(r => r.json())
//     const peer = new Peer({ initiator: false, trickle: false })
//
//     peer.signal({ type: 'offer', sdp: offerRes.sdp })
//
//     peer.on('signal', async answer => {
//         await fetch('/rtc/answer', {
//             method: 'POST',
//             headers: { 'Content-Type': 'application/json' },
//             body: JSON.stringify({ id: offerRes.id, sdp: answer.sdp }),
//         })
//     })
//
//     peerInstance = await new Promise(resolve => {
//         peer.once('connect', () => resolve(peer))
//     })
//
//     return peerInstance
// }
//
// export function sendUnaryRequest(service, method, payload) {
//     if (!peerInstance) throw new Error("Peer not connected")
//
//     const id = uuidv4()
//     const msg = {
//         id,
//         service,
//         method,
//         mode: 'unary',
//         payload,
//     }
//
//     peerInstance.send(JSON.stringify(msg))
//
//     return new Promise(resolve => {
//         peerInstance.on('data', raw => {
//             const data = JSON.parse(raw.toString())
//             if (data.id === id) resolve(data)
//         })
//     })
// }