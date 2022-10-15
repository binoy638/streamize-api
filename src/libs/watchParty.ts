/* eslint-disable unicorn/no-null */
export interface Member {
  id: string; // socket id
  name: string;
  isHost: boolean;
  isWatching: boolean; // keep track of whether the member is watching the stream
}

export interface PlayerInfo {
  video: {
    torrentID: string;
    slug: string;
    title: string;
  };
  currentTime: number;
  totalDuration: number;
  isPlaying: boolean;
}

// data required to create a watch party
export interface PartyConstructor {
  name: string;
  slug: string;
  maxViewers: number;
  partyPlayerControl: boolean;
  host: string;
}

export interface RoomInfo {
  slug: string;
  name: string;
  maxViewers: number;
  partyPlayerControl: boolean;
  host: string;
  members: Member[];
  playerInfo: PlayerInfo | null;
}

class WatchParty {
  public name: string;

  public slug: string;

  public maxViewers: number;

  public partyPlayerControl: boolean;

  public host: string;

  public members: Member[];

  public playerInfo: PlayerInfo | null;

  constructor(party: PartyConstructor) {
    this.name = party.name;
    this.slug = party.slug;
    this.maxViewers = party.maxViewers;
    this.partyPlayerControl = party.partyPlayerControl;
    this.host = party.host;
    this.members = [];
    this.playerInfo = null;
  }

  public addMember(member: Member): void {
    this.members.push(member);
  }

  public removeMember(member: Member): void {
    this.members = this.members.filter(m => m.id !== member.id);
  }

  public getMember(memberId: string): Member | undefined {
    return this.members.find(m => m.id === memberId);
  }

  public getRoomInfo(): RoomInfo {
    return {
      name: this.name,
      slug: this.slug,
      maxViewers: this.maxViewers,
      partyPlayerControl: this.partyPlayerControl,
      host: this.host,
      members: this.members,
      playerInfo: this.playerInfo,
    };
  }

  public allowPlayerControl(bool: boolean): void {
    this.partyPlayerControl = bool;
  }

  public setPlayerInfo(playerInfo: PlayerInfo): void {
    this.playerInfo = playerInfo;
  }

  public getPlayerInfo(): PlayerInfo | null {
    return this.playerInfo;
  }

  public setVideoCurrentTime(time: number): void {
    if (this.playerInfo) {
      this.playerInfo.currentTime = time;
    }
  }

  public getVideoCurrentTime(): number | undefined {
    if (this.playerInfo) {
      return this.playerInfo.currentTime;
    }
    return undefined;
  }

  public setIsPlaying(bool: boolean): void {
    if (this.playerInfo) {
      this.playerInfo.isPlaying = bool;
    }
  }
}

export default WatchParty;
