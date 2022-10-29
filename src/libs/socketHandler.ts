import type { Socket } from 'socket.io';
import logger from '../config/logger';
import { WatchPartyModel } from '../models/watchParty';
import WatchParty, { IPlayer, Member, PartyConstructor, Player, RoomInfo } from './watchParty';

enum RoomEvent {
  // emitted when a user tries to join a room
  MEMBER_JOIN = 'join',
  MEMBER_LEAVE = 'leave',
  PLAY = 'play',
  PAUSE = 'pause',
  RESUME = 'resume',
  SEEK_TO = 'seekTo',
  CURRENT_TIME_UPDATE = 'currentTimeUpdate',
  PLAYER_UPDATE = 'playerUpdate',

  // client events

  MEMBER_JOINED = 'joined', // emitted when a user joins a room successfully
  MEMBER_LEFT = 'left', // emitted when a user leaves a room
  PLAYED = 'played',
  RESUMED = 'resumed',
  SEEKED_TO = 'seekedTo',
  PAUSED = 'paused',
  CURRENT_TIME_UPDATED = 'currentTimeUpdated',
  PLAYER_UPDATED = 'playerUpdated',
}

enum RoomErrorEvent {
  ROOM_NOT_FOUND = 'room-not-found',
}

interface JoinRoomData {
  slug: string;
  member: Member;
}

interface ServerToClientEvents {
  [RoomEvent.MEMBER_JOINED]: (data: RoomInfo) => void;
  [RoomErrorEvent.ROOM_NOT_FOUND]: (data: { slug: string }) => void;
  [RoomEvent.PLAYER_UPDATED]: (data: IPlayer) => void;
  [RoomEvent.PLAYED]: () => void;
  [RoomEvent.PAUSED]: () => void;
  [RoomEvent.RESUMED]: () => void;
  [RoomEvent.SEEKED_TO]: (data: { slug: string; seekTime: number }) => void;
  [RoomEvent.MEMBER_LEFT]: (data: Member) => void;
}

interface ClientToServerEvents {
  [RoomEvent.PLAYER_UPDATE]: (data: { slug: string; playerInfo: IPlayer }) => void;
  [RoomEvent.PLAY]: (data: { slug: string }) => void;
  [RoomEvent.PAUSE]: (data: { slug: string }) => void;
  [RoomEvent.RESUME]: (data: { slug: string }) => void;
  [RoomEvent.SEEK_TO]: (data: { slug: string; seekTime: number }) => void;
  [RoomEvent.MEMBER_JOIN]: (data: JoinRoomData) => void;
  [RoomEvent.CURRENT_TIME_UPDATE]: (data: { slug: string; currentTime: number }) => void;
}

// INFO: it contains all the watch parties
const ROOMS = new Map<string, WatchParty>();

const getRoomPlayer = (slug: string): Player => {
  const room = ROOMS.get(slug);
  if (!room) {
    logger.error(`room not found: ${slug}`);
    throw new Error('room not found');
  }

  const player = room.getPlayer();

  if (!player) {
    throw new Error('player not found');
  }

  return player;
};

const socketHandler = (socket: Socket<ClientToServerEvents, ServerToClientEvents>): void => {
  logger.info(`a user connected : ${socket.id}`);

  socket.on(RoomEvent.MEMBER_JOIN, async data => {
    const { slug, member } = data;
    logger.debug(`member join: ${slug}`);
    const roomData = await WatchPartyModel.findOne({ slug });

    if (!roomData) {
      socket.emit(RoomErrorEvent.ROOM_NOT_FOUND, { slug });
      logger.debug(`room not found: ${slug}`);
      socket.disconnect();
      return;
    }

    // INFO: if the room is already present in the map, then add the member to the room otherwise create a new room
    let room = ROOMS.get(slug);
    if (!room) {
      logger.error(`room not found: ${slug}`);
      logger.info(`creating a new room: ${slug}`);
      const roomConstructor: PartyConstructor = {
        slug,
        host: String(roomData.host),
        name: roomData.name,
        maxViewers: roomData.maxViewers,
        partyPlayerControl: roomData.partyPlayerControl,
      };
      room = new WatchParty(roomConstructor);
      ROOMS.set(slug, room);
    }

    // Initially the is will contain the host id so that we can check if the host is the one who is joining the room

    const isHost = String(roomData.host) === member.id;

    room.addMember({ ...member, isHost, id: socket.id });
    socket.join(slug);

    const { roomInfo } = room;

    socket.to(slug).emit(RoomEvent.MEMBER_JOINED, roomInfo);
  });

  // socket.on(RoomEvent.MEMBER_LEFT, async (data: JoinRoomData) => {
  //   const { slug } = data;

  //   const roomData = await WatchPartyModel.findOne({ slug });

  //   if (!roomData) {
  //     socket.emit(RoomErrorEvent.ROOM_NOT_FOUND, { slug });
  //     socket.disconnect();
  //     return;
  //   }

  //   const room = ROOMS.get(slug);
  //   if (!room) {
  //     logger.error(`room not found: ${slug}`);
  //     return;
  //   }
  //   room.removeMember(socket.id);

  //   socket.to(slug).emit(RoomEvent.MEMBER_LEFT);
  // });

  socket.on(RoomEvent.PLAYER_UPDATE, data => {
    try {
      const { slug, playerInfo } = data;

      const player = getRoomPlayer(slug);

      player.playerInfo = playerInfo;

      logger.debug(`player updated: ${JSON.stringify(player.playerInfo)}`);

      socket.to(slug).emit(RoomEvent.PLAYER_UPDATED, player.playerInfo);
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on(RoomEvent.PLAY, (data: { slug: string }) => {
    try {
      const { slug } = data;
      const player = getRoomPlayer(slug);

      player.status = 'playing';

      logger.debug('player played');

      socket.to(slug).emit(RoomEvent.PLAYED);
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on(RoomEvent.PAUSE, (data: { slug: string }) => {
    try {
      const { slug } = data;
      const player = getRoomPlayer(slug);

      player.status = 'paused';

      logger.debug('player paused');

      socket.to(slug).emit(RoomEvent.PAUSED);
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on(RoomEvent.RESUME, (data: { slug: string }) => {
    try {
      const { slug } = data;
      const player = getRoomPlayer(slug);

      player.status = 'playing';

      logger.debug('player resumed');

      socket.to(slug).emit(RoomEvent.RESUMED);
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on(RoomEvent.SEEK_TO, (data: { slug: string; seekTime: number }) => {
    try {
      const { slug } = data;
      const player = getRoomPlayer(slug);

      if (!player) {
        throw new Error('player not found');
      }

      logger.debug('player seeked');

      socket.to(slug).emit(RoomEvent.SEEKED_TO, data);
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on(RoomEvent.CURRENT_TIME_UPDATE, (data: { slug: string; currentTime: number }) => {
    try {
      const { slug, currentTime } = data;
      const player = getRoomPlayer(slug);

      player.currentTime = currentTime;

      logger.debug('player current time updated');
    } catch (error) {
      logger.error(error);
    }
  });

  socket.on('disconnect', () => {
    logger.info(`a user disconnected : ${socket.id}`);

    // eslint-disable-next-line unicorn/no-array-for-each
    ROOMS.forEach(room => {
      // remove the member from the room array and emit a event to the clients of the room
      const member = room.getMember(socket.id);
      if (member) {
        logger.info(`member left: ${member.id}`);
        room.removeMember(socket.id);
        socket.to(room.slug).emit(RoomEvent.MEMBER_LEFT, member);
      }

      // if room is empty then remove the room from the map
      if (room.members.length === 0) {
        logger.info(`deleting empty room: ${room.slug}`);
        ROOMS.delete(room.slug);
      }
    });
  });
};

export default socketHandler;
