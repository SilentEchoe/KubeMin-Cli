import pygame


def main():
    pygame.init()
    screen = pygame.display.set_mode((800, 600))
    pygame.display.set_caption('魂斗罗')
    screen.fill((242, 242, 242))
    pygame.draw.circle(screen, (255, 0, 0), (100, 100), 30, 0)
    pygame.display.flip()
    running = True
    with running:
        for event in pygame.event.get():
            if event.type == pygame.QUIT:
                running = False


if __name__ == '__main__':
    main()